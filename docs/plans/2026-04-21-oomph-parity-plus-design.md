# Oomph 대비 탐지 권위성 추월 설계 (v1.0-rc target)

- **상태**: Draft → 승인됨 (2026-04-21)
- **대상 버전**: Better-pm-AC v1.0-rc
- **선행 릴리스**: v0.10.0-beta (17 체크 β + sim/world/entity/ack 골격)
- **런타임 타깃**: PMMP 5.x 최신 stable, MCBE 1.21.x 최신 프로토콜 (Gophertunnel 상위 호환)
- **접근법**: B — 현 골격을 권위 모델로 in-place 업그레이드, Oomph는 참조 자료로만 활용

---

## §1. 목표 & 성공 지표

**목표.** Better-pm-AC는 **PMMP(PocketMine-MP 5.x) 앞단에 붙는 Bedrock Edition 전용 MiTM 안티치트 프록시**다. 이 계획서의 목표는, Oomph가 증명한 server-authoritative + dual-tick 모델의 탐지 권위성을 β에 이미 쌓인 sim·rewind·ack 골격 위에 승격시키되, **PMMP 환경 특유의 약점(플러그인-AC 지연, PMMP 자체의 제한적 이동 권위성)을 프록시 레이어에서 대리 수행하는 포지셔닝**으로 확립하고, 동시에 Oomph가 exempt하거나 불완전한 네 개 영역(액체 / 작은 Δ 충돌 / 블록 bbox / mitigation alerting)을 moat로 유지해 false-positive를 선두로 끌어내리는 것.

**측정 가능한 성공 지표 (v1.0-rc 기준).**

| 축 | 지표 | 목표 |
|---|---|---|
| 권위성 | Movement sim-truth 적용 체크 비율 | ≥ 80% (현재 0%: 전부 validate-only) |
| 권위성 | Combat multi-raycast 통합 여부 | Reach/A · KillAura/A·B 전부 multi-ray |
| PMMP 호환 | 실제 PMMP 5.x 대상 시나리오 | 10개 PMMP 플러그인 환경(survival, skyblock, minigame hub 등) green |
| 품질 (FP) | 내부 legit 시나리오 회귀 제로 | 8 → 20개로 확장, 전부 클린 |
| 품질 (FP) | PCAP 재생 하네스 (PMMP 기록) flag/hour | < 0.1 |
| 품질 (FN) | 알려진 치트(Horion/Zephyr/Latite) 재생 | 범주별 ≥ 1회 flag 보장 |
| 완결성 | Oomph 약점 4축 회귀 테스트 | 액체/Δ/bbox/alerting 모두 green |
| 관측성 | per-check 메트릭 breakdown | 모든 체크에 `{type,subtype,policy}` 라벨 |
| 성능 | p99 OnInput 100 CCU | β 7.8 µs/player 유지 또는 10 µs 이하 |
| 운영 이식성 | PMMP 운영자 배포 경로 | Docker sidecar + systemd unit + `pocketmine.yml` 공존 예시 |

**비목표.** ML/neural 탐지, 상용 SaaS, GUI 관리자 패널, 순정 Bedrock dedicated server 전용 최적화. 로그인 fingerprint와 world-block 체크(Nuker/FastBreak)는 유지하되 **우선순위 2순위**로 내려 권위 모델 완성 이후에 붙인다.

**실패 조건.** 위 지표를 8주 안에 모두 green으로 만들 실현 가능한 경로가 보이지 않으면 이 계획서는 실패 — 범위를 줄이거나 접근법 A(Oomph 아키텍처 포크)로 전환.

---

## §2. 도메인 모델 — Dual-Tick & Authoritative Position

**핵심 변경.** β의 "client pos = truth" → **"sim pos = truth, client pos = claim"** 로 의미 전환. 이걸 깔끔히 표현하려면 지금 암묵적으로 섞여 있는 두 개 시계를 분리하고, 플레이어 위치를 세 개 상태로 쪼갠다.

### 2.1 두 개의 시계

| 시계 | 의미 | 출처 | β 상태 |
|---|---|---|---|
| `ServerTick` | 프록시의 monotonic tick (20 TPS 기준 50 ms 고정) | 프록시 자체 ticker | 없음 — check가 client tick으로 대체 |
| `ClientTick` | 클라가 claim하는 tick | `PlayerAuthInput.Tick` | ack queue key로만 사용 |

**관계.** 정상 네트워크에서 `ClientTick ≤ ServerTick` (RTT/2 만큼 뒤처짐). 역전·급증 = BadPacket/A 또는 Timer/A 신호. `TickSkew := ServerTick − ClientTick` 를 per-player 상태로 트래킹 → 이후 rewind·ack·multi-raycast 모두 이 skew로 좌표계를 맞춤.

### 2.2 위치 3-상태 모델

플레이어 상태는 매 tick마다 다음 셋을 갖는다:

- **`ExpectedPos`** — sim이 `prevCommittedPos + inputs + effects + collision` 으로 계산한 결과. **Truth.**
- **`ClaimedPos`** — `PlayerAuthInput.Position` 으로 클라가 주장. **Unvalidated.**
- **`CommittedPos`** — 매 tick 끝에 수락된 최종 위치. 다음 tick의 `prevCommittedPos`.

### 2.3 Reconciliation 규칙 (매 OnInput)

1. sim을 돌려 `ExpectedPos` 산출.
2. `Δ = ClaimedPos − ExpectedPos`, `|Δ|` 계산.
3. 분기:
   - `|Δ| ≤ ε_accept` (기본 0.03 b, 부동소수 오차 + PMMP 5.x round-trip 관용치) → `CommittedPos = ClaimedPos`. 정상.
   - `ε_accept < |Δ| ≤ ε_pending` (기본 0.3 b) → **pending**. 이 tick에 서버가 보낸 knockback/텔레포트/correction이 아직 ack 안 된 경우일 수 있음. Ack queue 조회 후 해당 action이 pending이면 `CommittedPos = ClaimedPos`, 아니면 divergence 카운트 1 증가.
   - `|Δ| > ε_pending` → **authoritative snap**. `CommittedPos = ExpectedPos`, `CorrectPlayerMovePrediction` 패킷을 클라로 발사해 클라 위치 강제 교정. 체크(Speed/Fly/Phase/etc.)에게 "이번 tick은 sim-truth" 시그널.
4. 각 체크는 `CommittedPos` 기반으로만 판정 → 체크 간 불일치 원천 제거.

### 2.4 Authoritative Correction 채널 — `CorrectPlayerMovePrediction`

β의 rubberband은 `MovePlayer + MoveModeReset` 조합이었으나, **Bedrock 프로토콜이 server-auth를 위해 공식 제공하는 `CorrectPlayerMovePrediction`** 로 전환. 이점:
- 클라가 prediction rollback을 수용하도록 프로토콜 수준에서 디자인됨 → 시각적 rubberband jitter 최소.
- `PredictionType` 필드로 `Player` / `Vehicle` 분리 → 차량/elytra 상황 일관 처리.
- Oomph도 이 패킷 사용. PMMP 5.x가 이 패킷을 직접 생성하지 않으므로(플러그인 지원이 부분적) **우리 프록시가 이 권위 채널을 "PMMP 대리"로 독점**.

### 2.5 Ack Queue 의미 확장

β의 ack queue는 `{clientTick → pending action}` 이었음. 확장:
- 키를 `{serverTick, clientTick}` 복합으로 변경. server 쪽에서 action을 발행한 시점(= serverTick)과 클라가 확인해야 할 시점(= 예상 clientTick = serverTick − skew) 둘 다 기록.
- 항목에 `ExpectedDelta Vec3` 추가 — 그 action이 실행됐을 때 sim이 예측한 Δpos. Reconciliation의 `ε_pending` 분기에서 "pending action의 ExpectedDelta가 Claimed Δ와 유사한가?" 로 일치 여부 판정.

### 2.6 Manager API 변경 (요약)

```go
// β: m.OnInput(pid, tick, pos, onGround, yaw, pitch, inputMode, inputData)
// v1.0-rc:
m.OnInput(pid, clientTick, claimedPos, onGround, yaw, pitch, inputMode, inputData)
   → Manager 내부에서 serverTick 붙이고 sim 돌려 committedPos 확정 후
     체크 슬라이스에 (player, tickCtx) 주입.
```

`data.Player` 상태 추가: `ExpectedPos`, `ClaimedPos`, `CommittedPos`, `PrevCommittedPos`, `TickSkew`, `LastCorrectionTick`.

### 2.7 PMMP 상호작용

**중요 제약.** 우리가 `CorrectPlayerMovePrediction` 를 쏴도 PMMP 쪽은 여전히 클라가 다음 tick에 보낼 `PlayerAuthInput` 을 기준으로 state를 유지함. 이 엇박자를 막으려면 **upstream(프록시 → PMMP) 방향 `PlayerAuthInput` 에 `CommittedPos` 를 덮어써 전달**:

- 클라 → 프록시: `ClaimedPos` (우리 검증 대상)
- 프록시 → PMMP: `CommittedPos` (PMMP 기준 상태)

이게 진짜 "프록시가 PMMP 앞에서 이동 권위 레이어를 대리 수행한다" 의 실체. PMMP 플러그인(자체 anti-cheat 포함)도 `CommittedPos` 만 보게 되므로 오탐 이중화 방지.

---

## §3. 컴포넌트 변경 맵

| 모듈 | 변경 규모 | 핵심 변경 |
|---|---|---|
| `proxy/` | 중 | `CorrectPlayerMovePrediction` 송신 경로 추가. Upstream `PlayerAuthInput.Position` 을 `CommittedPos` 로 rewrite. `ServerFilterFunc` 를 실제 패킷 파이프라인에 배선 (β에서 nil). |
| `anticheat/` (Manager) | 대 | `ServerTick` ticker 추가. `OnInput` 시그니처에 `clientTick` 명시. sim → reconcile → check 순서로 파이프라인 재편. `TickContext{ServerTick, ClientTick, Skew}` 를 check에 주입. |
| `anticheat/data/Player` | 대 | `ExpectedPos / ClaimedPos / CommittedPos / PrevCommittedPos / TickSkew / LastCorrectionTick` 추가. 체크가 개별로 갖던 pos 필드 전부 제거 → 단일 소스. |
| `anticheat/meta` | 소 | `Detection.Check` 시그니처에 `TickContext` 추가. 새 플래그 `Authority() Authority` — `Validate` / `AuthoritativeMovement` / `AuthoritativeCombat`. |
| `anticheat/sim` | 중 | API 순화: `Step(PrevCommitted, Inputs, Effects, World) → ExpectedPos` 순수 함수. Oomph 약점 메우는 추가: lava-specific drag, bubble column, small-Δ 충돌 resolver(< 1e-4 b), soul sand / powder snow. |
| `anticheat/world` | 중 | **Block BBox 완결성** — trapdoor(open/closed/half), 펜스/벽 연결, 계단 7가지 모양, 더블 체스트, 하단/상단 슬랩, scaffolding stable/unstable 전부. Oomph bbox 갭 봉쇄. 블록-업데이트 히스토리는 인터페이스만 노출(γ+). |
| `anticheat/entity` | 중 | rewind 버퍼 항목에 `{pos, yaw, pitch, bbox}` 저장. Multi-raycast가 쓸 `EyePositionAt(rid, clientTick)` / `BBoxAt(rid, clientTick)` 노출. 창 크기 40→80틱(고RTT 플레이어). |
| `anticheat/ack` | 중 | 키를 `{ServerTick, ClientTick}` 복합으로 변경. 항목에 `ExpectedDelta Vec3` 추가. `Resolve(sTick, cTick, actualΔ) (matched bool)` 가 reconciliation의 pending 분기에서 호출. |
| `anticheat/mitigate` | 중 | `RubberbandFunc` 축소(낮은 confidence 경로 전용), 신규 `CorrectFunc(uuid, expectedPos, tick)` 가 `CorrectPlayerMovePrediction` 발사. per-check 메트릭 breakdown (`{type,subtype,policy}` 라벨). 새 메트릭 `better_pm_ac_corrections_total`, `better_pm_ac_reconcile_pending_total`, `better_pm_ac_reconcile_snap_total`. |
| `anticheat/checks/movement` | 대 | 전부 `CommittedPos + TickContext` 소비로 재작성. 개별 pos 추적·속도 계산 로직 제거. `Authority() = AuthoritativeMovement` 로 선언. |
| `anticheat/checks/combat` | 대 | `RayCaster` 헬퍼 신설 — `CastN(eyeFrom, eyeTo, windowTicks, targetRID) (hit bool, nearestT float32)`. Reach/A, KillAura/A·B 이 helper로 통일. `Authority() = AuthoritativeCombat`. |
| `anticheat/checks/packet` | 소 | 시그니처 변경만 흡수(TickContext). 로직 그대로. |
| `anticheat/checks/world` | γ+ | FastPlace/A(β WIP) 은 보류, 권위 모델 완성 후 재착수. |
| `anticheat/checks/client` | γ+ | Protocol/EditionFaker/ClientSpoof — 권위 모델과 독립이므로 병렬 가능하지만 우선순위 2. |
| `bench/` | 소 | 기존 벤치 유지 + `BenchmarkReconcile100CCU`, `BenchmarkMultiRaycast`. β의 7.8 µs/player 기준 회귀 가드. |
| `docs/check-specs/` | 중 | 기존 스펙 전부에 "§N.M Reconciliation 경로" 절 추가. 신규 `authoritative-movement-model.md`, `multi-raycast-combat.md`, `liquid-physics.md`, `bbox-coverage.md` 네 장 신설. |

### 3.1 파급이 가장 큰 한 곳 — `data.Player`

체크들이 지금은 각자 `prevPos`, `prevTick`, `lastHorizontalSpeed` 같은 상태를 중복 보유 중. 권위 모델로 가면 **이 전부가 Player 한 군데로 이동**. 체크는 read-only 관찰자가 됨. 이 단일-소스화가 체크 간 불일치(FP 원인 1위) 제거의 핵심.

### 3.2 `RubberbandFunc` 를 deprecate 아니라 축소로 두는 이유

`CorrectPlayerMovePrediction` 이 미지원인 예전 클라 빌드(1.19 이하)나 vehicle/elytra 상황에서 여전히 fallback으로 필요. 단일 교정 채널로 몰면 호환성 리스크. 둘을 **confidence level에 따라 분기** — high-confidence (sim divergence 명확) → `CorrectFunc`, low-confidence (ping spike 의심) → `RubberbandFunc`.

---

## §4. 체크별 재분류 & Multi-Raycast Combat

### 4.1 Movement — sim을 truth로 승격시키며 얻는 것

권위 모델 하에선 **sim 자체가 1차 방어선**이고 체크는 "divergence가 있었다" 는 신호를 남기는 센서. 같은 체크 이름을 유지하되 의미가 바뀐다:

| 체크 | β 의미 | γ 의미 (권위 모델) |
|---|---|---|
| Speed/A | 관측된 ∆x 수평속도 상한 | sim이 허용한 max horizontal Δ 와 ClaimedPos 차이. 넘치면 reconcile이 snap하고 체크는 누적 위반 카운트만 올림. |
| Fly/A | Y속도·GravViolTicks | sim이 gravity 적용했는데 Claimed Y가 떠있음 → reconcile pending/snap 이벤트를 그대로 signal. |
| NoFall/A | 낙하거리 vs damage 불일치 | sim 낙하거리가 truth. 서버 측 damage event와 sim 낙하거리가 벌어지면 flag. |
| Phase/A | ClaimedPos가 블록 내부 | sim collision resolver가 block내부를 거부했는데 Claimed은 거기 있음 → snap. |
| NoSlow/A | 아이템 사용 중 속도 | sim이 item-use speed modifier 적용한 ExpectedPos 대비 Δ. |
| Velocity/A | server knockback vs claimed Δ | ack queue의 ExpectedDelta와 actual Δ 매칭. 불일치 누적. |

**핵심 결과**: 체크의 로컬 상태(prevPos, lastTick, ...)가 전부 `data.Player` 로 이동 → 체크는 **stateless 관찰자**. false-positive의 1위 원인인 "체크 간 pos 불일치" 소멸.

### 4.2 Combat — Multi-Raycast 헬퍼

Bedrock 클라가 hit 판정할 때 단일 ray가 아니라 **attack animation span(~2틱)에 걸쳐 여러 ray를 쏘고 최소거리 taker** 를 쓴다. 우리가 rewind 위에서 같은 일을 하지 않으면 pre-swing 공격을 과잉 flag하거나 post-swing 공격을 놓친다.

새 헬퍼 `anticheat/combat/raycaster.go`:

```go
type RayCaster struct {
    entities *entity.Tracker
    world    *world.Tracker
}

type HitResult struct {
    Hit        bool
    NearestT   float32   // 0..1, 가장 가까웠던 sample tick에서의 ray 파라미터
    SampleTick uint64
}

// CastN은 client tick 기준 ±window 틱 샘플들에 대해 eye→forward ray를
// 타깃 bbox와 교차 검사. reach 초과 시 Hit=false.
func (rc *RayCaster) CastN(
    playerID uuid.UUID,
    targetRID uint64,
    clientTick uint64,
    window uint64,          // 기본 2
    maxReach float32,        // 기본 3.1
) HitResult
```

이 헬퍼 위에 Reach/A, KillAura/A, KillAura/B를 재작성. KillAura/C(한 틱 다중 타깃)·AutoClicker·Aim 은 timestamp 기반이라 그대로.

### 4.3 Packet / World / Client

- **Packet** — `TickContext` 시그니처만 흡수, 로직 그대로. **우리 moat 유지 지점**.
- **World** (Nuker/FastPlace/…) — 권위 모델 완성 후. block-update 히스토리는 §3 world 표 대로 인터페이스만 노출.
- **Client** (Protocol/EditionFaker/ClientSpoof) — 권위 모델과 독립, 병렬 진행 가능. §7 Phase γ.6에 배치.

---

## §5. Oomph 약점 → 우리 moat

네 축 전부 회귀 테스트 게이트로 고정:

1. **액체 커버리지.** 우리 sim 이미 water drag·buoyancy 구현 중. 추가: bubble column 상승 drag, kelp/seagrass full-body slow, lava 전용 drag 계수(water와 다름). `docs/check-specs/liquid-physics.md` 신설. Oomph가 "the client is fully exempted" 라고 README에 적어둔 그 영역을 **우리 체크는 전부 돈다**는 점을 공식 문서화.
2. **작은 Δ 충돌.** `sim/collision.go` 에 precision tier 도입 — 1e-4 b 이하는 bitwise 비교, 이상은 기존 axis-sweep. block-seam 마이크로 관통 치트 봉쇄.
3. **Block BBox 완결성.** `docs/check-specs/bbox-coverage.md` 에 non-cube 블록 전수표(trapdoor 4상태 × open/closed, 펜스/벽 연결 16, 계단 7, 더블 체스트, 슬랩 상/하, scaffolding, 일반·이상 모양). 각 엔트리마다 world 단위 테스트.
4. **Mitigation alerting.** `docs/ops-runbook.md` 확장 — reconcile snap rate, correction rate, tick skew p99 알림 threshold. Grafana 대시보드 JSON 유지 + per-check panel 추가.

---

## §6. 관측성 & Replay Harness

### 6.1 Per-check 메트릭 breakdown

expvar는 레이블 없는 counter만 지원 → **expvar.Map** 으로 `{type}/{subtype}/{policy}` 키 계층을 직접 세팅. Prometheus 의존성 피함(β 방침 유지).

신규 카운터:
- `better_pm_ac_violations_by_check` (Map)
- `better_pm_ac_corrections_total` (new)
- `better_pm_ac_reconcile_pending_total`
- `better_pm_ac_reconcile_snap_total`
- `better_pm_ac_tick_skew_samples` (Map, p50/p99)
- `better_pm_ac_oninput_latency_ns` (Int, running histogram via sparse buckets)

### 6.2 Replay harness

`cmd/replay/` 신설. 포맷: 프레임 바이너리 `{uint64 ts_ns, uint8 dir, uint32 len, bytes pkt}`. 녹화 모드(`--record path.bpac`)는 프록시가 살아있는 동안 모든 클라/서버 방향 패킷 dump. 재생 모드는 Manager만 부트하고 파일 스트림. 기대 출력(= 녹화 당시 체크 결과)을 골든 fixture로 둬 비교. CI 회귀 게이트.

v1 스코프는 **개별 시나리오 골든 5개**(idle / walk / sprint+jump / combat exchange / fall). 실제 PMMP 라이브 녹화는 v1.1로 슬립.

---

## §7. 페이즈 순서 & 피처 플래그

**8주 페이즈드 롤아웃**, 각 페이즈 끝에 shippable 커밋·태그.

| Phase | 기간 | 내용 | 게이트 |
|---|---|---|---|
| γ.1 | W1–2 | ServerTick ticker, `data.Player` 위치 3-상태, `sim.Step()` 순수화, `TickContext` 전파 | β 테스트 전부 green (shadow 모드, 체크 로직 변경 없음) |
| γ.2 | W2–3 | Reconcile 3-분기, `CorrectPlayerMovePrediction` 송신, upstream `PlayerAuthInput` rewrite | 내부 scenario 20개 중 legit 10개 FP=0 |
| γ.3 | W3–5 | Movement 체크 6개 권위 모델로 재작성, `authority.movement` 플래그 canary → 100% | reconcile snap rate 내부 테스트에서 예측 범위 |
| γ.4 | W5–6 | `RayCaster` 헬퍼, Reach/A·KillAura/A·B 재작성, entity rewind 확장(yaw/pitch/bbox) | 치트 재생(Horion/Zephyr/Latite) 범주별 flag ≥ 1 |
| γ.5 | W6–7 | Oomph moat 강화(액체/작은Δ/bbox), per-check 메트릭, replay harness v1 | v1.0-rc 성공 지표 표 전부 green |
| γ.6 | W7–8 | World-block 체크(Nuker/FastBreak/FastPlace/Tower/InvalidBreak), client fingerprint 3종 | 각 체크 스펙 + 단위/시나리오 테스트 커버 |

**피처 플래그** (config.toml `[anticheat.authority]`):
- `dual_tick` (γ.1에서 on)
- `reconcile_snap` (γ.2에서 canary → on)
- `upstream_rewrite` (γ.2에서 canary → on; PMMP plugin 충돌 의심 시 off)
- `movement_auth` (γ.3에서 canary → on)
- `combat_multiray` (γ.4에서 canary → on)

각 플래그 off ↔ on 전환은 **메트릭 비교 후**. off = 기존 β 동작, on = γ 권위 모델.

---

## §8. 테스트 전략

- **Unit** — 체크별 table-driven (현재 기조 유지). TickContext를 포함한 fake 생성기 추가.
- **Scenario integration** — legit 3→12 (idle/walk/sprint/jump/swim/lava/climb/fall/knockback/push/elytra/boat), cheat 5→15 (+Scaffold/Tower/Nuker/FastPlace/Phase 마이크로/Speed 미세/etc.).
- **Replay golden** — §6.2의 5개 시나리오 골든 fixture CI 회귀 게이트.
- **PMMP-in-a-box** — `test/pmmp-compose/` : docker-compose로 PMMP 5.x 최신 + 프록시 + 스모크 테스트 클라(Gophertunnel bot). 로컬/nightly CI에서 실행.
- **Bench** — β 벤치 유지 + `BenchmarkReconcile100CCU`, `BenchmarkMultiRaycast`. p99 OnInput ≤ 10 µs/player 게이트.
- **Fuzz** — `go test -fuzz` : `PlayerAuthInput` bitset·pos·tick 필드. BadPacket 체크들이 panic 없이 처리하는지 보장.

---

## §9. 리스크 & 대응

| 리스크 | 영향 | 대응 |
|---|---|---|
| dual-tick 리팩터가 β 체크 회귀 유발 | 고 | Phase γ.1 전부 shadow 모드(체크 로직 변경 금지). 플래그 off = β 동작 유지. |
| `CorrectPlayerMovePrediction` 미지원 구버전 MCBE | 중 | `RubberbandFunc` fallback 유지. MCBE 버전별 테스트 매트릭스. |
| PMMP 플러그인과 upstream rewrite 충돌 | 중 | `authority.upstream_rewrite=false` 탈출구. PMMP 운영진용 "compat 모드" 문서화. |
| 8주 타임라인 공격적 | 중 | γ.5–.6는 slip 가능(moat 강화와 γ deferred). γ.1–.4가 must-have. |
| sim 정확성 회귀 | 고 | sim.Step() 순수 함수화 → property-based test 가능. lava/bubble 등 추가 feature는 스펙 문서 먼저, 그 다음 PR. |
| 메트릭 cardinality 폭발 | 저 | `{type,subtype,policy}` 셋으로 라벨 고정, 임의 플레이어 ID 라벨 금지. |
| 치트 재생 코퍼스 부재 | 중 | γ.4 전에 Horion/Zephyr/Latite 테스트 바이너리로 자체 녹화. 공개 불가 시 해시만 커밋. |

---

## 참고

- [Oomph](https://github.com/oomph-ac/oomph) — Go Bedrock AC 프록시. 본 설계의 참조 자료.
- [Gophertunnel](https://github.com/Sandertv/gophertunnel) — Bedrock 프로토콜 Go 구현체.
- [PocketMine-MP](https://github.com/pmmp/PocketMine-MP) — 타깃 서버 런타임.
- `docs/plans/2026-04-19-anticheat-overhaul-design.md` — β overhaul 설계 (선행 작업).
- `docs/plans/2026-04-19-work-board.md` — β work board (현재 상태 추적).
- `CHANGELOG.md` — v0.10.0-beta 릴리스 노트의 "Known limitations" 섹션.
