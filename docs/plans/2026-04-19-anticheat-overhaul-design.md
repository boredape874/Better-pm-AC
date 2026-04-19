# Better-pm-AC 전면 재설계 — Server-Authoritative Anti-Cheat

> **문서 위치**: `docs/plans/2026-04-19-anticheat-overhaul-design.md`
> **상태**: 설계 확정 · 구현 착수 단계
> **최종 수정**: 2026-04-19
> **작성 컨텍스트**: 사용자 지시로 Oomph 기반 MiTM 안티치트를 PMMP 환경에서 Oomph를 능가하도록 전면 재설계. 다중 AI 협업 작업용 단일 진실 원천(SSoT) 문서.

## 0. 이 문서의 사용법 (새 AI를 위한 온보딩)

**이 저장소에 처음 들어오는 AI라면 아래 순서로 읽고 행동하세요:**

1. **이 문서(`design.md`) 전체를 끝까지 읽기.** §0–§11까지 모두 필수. 건너뛰면 인터페이스 위반·파일 충돌이 발생합니다.
2. **[AGENT-PROTOCOL.md](./2026-04-19-agent-protocol.md)를 읽기.** 파일 경계·Task claim 절차·커밋 규약이 있습니다.
3. **[WORK-BOARD.md](./2026-04-19-work-board.md)를 열기.** 자신의 역할(AI-W/S/E/A/M/C/O)에 해당하는 Phase의 `pending` Task를 찾아 claim.
4. **작업 시작.** 패키지 경계(§AGENT-PROTOCOL §2)를 절대 넘지 마세요.
5. **완료 시 WORK-BOARD 업데이트 + PR 제출.** design.md는 설계 변경이 있을 때만 Orchestrator(AI-O)가 수정.

**이 문서에 적힌 결정은 사용자가 Q&A로 확정한 사항**이며, 임의 변경 금지. 이견이 있으면 WORK-BOARD에 `Proposal` 항목으로 적고 Orchestrator의 승인을 기다리세요.

---

## 1. 프로젝트 컨텍스트

### 1.1 목표

**한 문장**: PMMP(PocketMine-MP) 백엔드 서버 앞에 앉는 Go MiTM 프록시 형태의 Bedrock Edition 안티치트를, **Oomph를 능가하는 수준**으로 재설계·구현한다.

"능가"의 구체 정의:
- **검출 깊이**: Oomph와 같은 서버 권위 movement/combat 검증 + 물리 시뮬레이션 풀스펙(γ).
- **커버리지**: Oomph ~40개 대비 **~44개 체크** (범위 2, §6 참조).
- **정확성**: 오탐(false positive)을 Oomph 수준 이하로 유지하면서 우회 저항성은 상회.

### 1.2 참고 저장소

| 저장소 | 용도 | 비고 |
|--------|------|------|
| [oomph-ac/oomph](https://github.com/oomph-ac/oomph) | **1차 참고** — MiTM 구조, 서버 권위 검증 철학, 체크 네이밍 규약 | Go 기반, Dragonfly 라이브러리 사용 |
| [GrimAnticheat/Grim](https://github.com/GrimAnticheat/Grim) | 2차 참고 — 물리 시뮬레이션 기법, 체크 아이디어 | Java Edition 대상이라 직접 포팅 불가, 개념만 차용 |
| [aerisnetwork/oomph](https://github.com/aerisnetwork/oomph) | Oomph 포크, 커스텀 개선사항 참고 | 차이점은 `docs/forks-notes.md`에 AI-O가 정리 (Phase 1.5) |
| [TrixNEW/oomph](https://github.com/TrixNEW/oomph) | 동상 | 동상 |
| [basicengine92-tech/oomph](https://github.com/basicengine92-tech/oomph) | 동상 | 동상 |
| [df-mc/dragonfly](https://github.com/df-mc/dragonfly) | **의존성** — 청크 파싱·블록 BBox·월드 표현 재사용 | `go get github.com/df-mc/dragonfly` |
| [Sandertv/gophertunnel](https://github.com/Sandertv/gophertunnel) | 이미 사용 중 — Bedrock 프로토콜 구현 | go.mod에 기존재 |

### 1.3 환경 제약

- **백엔드**: PocketMine-MP 5.x (PHP). 프록시가 앞에서 RakNet 세션 중계.
- **프록시 언어**: Go 1.24.x.
- **개발 OS**: Windows 11 (champ@champ). 빌드 머신에서 `go build` 동작 필수.
- **런타임 OS**: Linux(배포), Windows(개발) 양쪽 지원.
- **타깃 동접자**: 100명 (Phase 1 성능 목표). 메모리 < 4GB.
- **Bedrock 프로토콜 버전**: gophertunnel `v1.55+`가 지원하는 범위. 최신 스테이블 따라감.

### 1.4 핵심 비기능 요구사항

| 항목 | 목표 |
|------|------|
| tick 처리 지연(p99) | < 2ms / player / tick |
| 동접자 | 100명 (Phase 1), 300명 (Phase 5) |
| 메모리/플레이어 | < 8MB (월드 캐시 포함 평균) |
| 시작 시간 | < 3초 |
| 바이너리 크기 | < 80MB |

---

## 2. 사용자 결정 사항 (Q&A 기록)

아래 5개 결정은 사용자와의 대화로 **확정**된 사항입니다. 변경하려면 Orchestrator를 통하세요.

### 2.1 결정 1 — 스코프: 풀 재설계 (옵션 A)

**질문**: 현재 휴리스틱 기반 구조를 유지하며 점진 보강할지, 전면 재설계할지.

**결정**: **전면 재설계**. 서버가 Bedrock 물리를 똑같이 돌려보고 결과를 비교하는 방식으로 간다.

**이유**: 현재 델타 임계값 방식은 숙련된 우회자에게 취약. 사용자는 Oomph를 능가하는 수준 요구.

### 2.2 결정 2 — 월드 데이터: Dragonfly 라이브러리 import (옵션 A1)

**질문**: 청크 데이터를 MiTM 스니핑으로 얻을지, PMMP 플러그인 컴패니언으로 얻을지, 하이브리드로 할지. 그리고 청크 파서를 자체 구현할지 Dragonfly 라이브러리를 import할지.

**결정**:
- **데이터 소스**: MiTM 스니핑 (서버→클라 `LevelChunk`/`SubChunk`/`UpdateBlock`/`UpdateBlockSynced` 가로채기). PMMP 플러그인 없이.
- **파서**: `github.com/df-mc/dragonfly` 라이브러리 import하여 청크·블록·BBox 로직 재사용.

**이유**: PMMP 백엔드 종속성을 피하고, Dragonfly의 검증된 청크 구현을 재사용해 유지보수 부담 최소화. Oomph와 동일 방식.

**오해 정리**: "Dragonfly import"는 "PMMP를 Dragonfly로 바꾼다"는 뜻이 아님. Go 라이브러리로만 사용, 백엔드 서버는 그대로 PMMP.

### 2.3 결정 3 — 물리 충실도: γ 풀스펙 (β→γ 단계 출시)

**질문**: 물리 시뮬레이션을 어디까지 재현할지.

**결정**: **γ 풀스펙** (엘리트라·릿프타이드·윈드차지·가루눈·꿀·거미줄·스캐폴딩 블록·슬라임 바운스 체인 전부). 단, 출시는 **β→γ 2단계**:
- **β 마일스톤**: 걷기·뛰기·점프·중력·기본 블록 충돌·얼음·점액·물·사다리·덩굴·포션 효과. 이 지점에서 Oomph 패리티 달성.
- **γ 마일스톤**: β 이후 엘리트라 등 특수 케이스 덧붙이기.

**이유**: 한 번에 γ 구현 시 버그 복합화로 검증 불가. β 단계에서 실전 배포 가능 상태 확보 후 γ 점진 확장.

### 2.4 결정 4 — Correction 정책: ④ 하이브리드

**질문**: 위반 감지 시 어떻게 제재할지. Kick-only / Client rubberband / Server filter / 하이브리드.

**결정**: **하이브리드** — 검사별로 policy 분기.

**매핑 원칙** (체크 구현 시 `Policy()` 메서드 반환값):
| 검사 범주 | Policy |
|----------|--------|
| Phase, Fly, Speed (위치 범주) | `ServerFilter` — PMMP에 유효한 위치로 덮어쓴 `MovePlayer` 전달 |
| NoFall, HighJump, Step | `ClientRubberband` — 클라만 되돌림 |
| Reach, KillAura, BadPacket, Timer | `Kick` — correction 무의미, VL 초과 시 즉시 추방 |
| AutoClicker, Aim, NoSlow, Nuker | `Kick` |
| Scaffold, Tower, FastPlace | `Kick` |
| Jitter류 경미 위반 | `ClientRubberband` |

**이유**: PMMP는 Dragonfly와 달리 프록시 correction을 재검증해 주지 않음. 일방적 rubberband은 월드 desync 위험. 검사 성격마다 적합한 대응이 다름.

### 2.5 결정 5 — 체크 커버리지: 범위 2 (~42 체크)

**질문**: Oomph 패리티(~34)/Oomph+α(~42)/완전체(~49) 중 어디까지.

**결정**: **범위 2** — Oomph + 18개 추가 = **총 44개**.

**제외된 카테고리**: Kaboom·CrashExploit·Spam·Bundle·KillAura/E — 검증 비용 대비 오탐 증가로 ROI 낮음.

상세 카탈로그는 §6 참조.

### 2.6 구현 기본값 (변경 가능)

아래는 질문하지 않은 항목이지만 **Orchestrator의 기본값**으로 설계에 반영. 변경 필요 시 WORK-BOARD에 `Proposal` 항목으로 제안.

| 항목 | 기본값 | 이유 |
|------|--------|------|
| Entity rewind window | **40 tick** (2초 @ 20 TPS) | Oomph 표준. 고지연(500ms) 수용 + 메모리 여유. |
| ACK 마커 구현 | `NetworkStackLatency` (서버→클라→서버 echo) | Oomph 방식. 별도 패킷 정의 불필요. |
| 동접자 목표 | 100명 (Phase 1), 300 (Phase 5) | §1.4 |
| 오탐 내성 | **보수적** (Conservative) | 사용자는 "PMMP 서버"용이라 플레이어 경험 우선 |
| 청크 캐시 전략 | **플레이어별** (공유 없음) | 구현 단순, 메모리 < 8MB/player 예산 충족 |
| 체크 Buffer 기본 | `FailBuffer=3, MaxBuffer=10` | 기존 구현 유지 |
| 로그 레벨 | `info` (위반은 `warn`, 디버그는 `debug`) | 기존과 동일 |

---

## 3. 현재 코드 분석

### 3.1 파일 구조 (before)

```
Better-pm-AC/
├── main.go
├── go.mod / go.sum
├── config/config.go
├── proxy/
│   ├── proxy.go          (448줄)
│   └── session.go        (50줄)
└── anticheat/
    ├── anticheat.go      (605줄, Manager)
    ├── meta/
    │   └── detection.go  (Detection 인터페이스, DetectionMetadata)
    ├── data/
    │   └── player.go     (898줄, 플레이어 상태)
    └── checks/
        ├── movement/  (speed, speed_b, fly, fly_b, nofall, nofall_b,
        │              noslow, phase, scaffold, timer, velocity)
        ├── combat/    (aim, aim_b, autoclicker, autoclicker_b,
        │              killaura, killaura_b, killaura_c, reach)
        └── packet/    (badpacket, badpacketb, badpacketc, badpacketd, badpackete)
```

### 3.2 기존 24 체크 분류

구현 후 각 체크의 운명은 **유지 / 개조 / 대체** 중 하나:

| 체크 | 분류 | 비고 |
|------|------|------|
| BadPacket/A (tick validation) | **유지** | 패킷 레이어 검증, 물리 무관 |
| BadPacket/B (pitch range) | **유지** | 동상 |
| BadPacket/C (sprint+sneak) | **유지** | 동상 |
| BadPacket/D (NaN/Inf) | **유지** | 동상 |
| BadPacket/E (contradictory flags) | **유지** | 동상 |
| Timer/A (packet rate) | **유지** | 패킷 타이밍 검증 |
| AutoClicker/A (CPS) | **유지** | 클릭 타이밍만 봄 |
| AutoClicker/B (stddev) | **유지** | 동상 |
| Aim/A (rounded yaw) | **유지** | 회전 패턴 분석 |
| Aim/B (const pitch) | **유지** | 동상 |
| KillAura/C (multi-target) | **유지** | 공격 이벤트 기반 |
| Scaffold/A (angle-based) | **개조** | Dragonfly BBox로 정밀도 상승 |
| Reach/A | **개조** | Entity Rewind 적용 (lag comp 정밀화) |
| KillAura/B (FOV) | **개조** | 동상 |
| NoSlow/A | **개조** | Sim 결과의 수평 속도와 대조 (기존 ceiling 비교 제거) |
| Velocity/A (Anti-KB) | **개조** | Sim이 knockback 후 기대 속도를 계산, 실측과 diff |
| Speed/A | **대체** → Speed/A (re-auth) | Sim 예측 vs 실제 diff (ceiling 비교 폐기) |
| Speed/B | **대체** → Speed/B (air) | 동상 |
| Fly/A | **대체** → Fly/A (sim hover) | 예측 Y와 실측 diff |
| Fly/B | **대체** → Fly/B (sim gravity) | 동상 |
| NoFall/A | **대체** → NoFall/A (sim damage expectation) | 낙하 데미지 이벤트 기대 |
| NoFall/B | **대체** → NoFall/B (onground spoof) | Sim onGround와 클라 claim 비교 |
| Phase/A | **대체** → Phase/A (sim collision) | BBox 충돌 기반 |
| KillAura/A (CPS vs swing) | **유지** | swing 이벤트 기반 |

유지 11 · 개조 5 · 대체 8 = 총 24.

### 3.3 data.Player 스키마 현재 상태

`anticheat/data/player.go`의 주요 필드:
- **위치**: `Position / LastPosition / Velocity / LastVelocity / OnGround / LastOnGround`
- **공중 상태**: `AirTicks / HoverTicks / GravViolTicks / GroundFallTicks`
- **낙하**: `FallDistance / LastFallDistance / FallStartY / fallTracking`
- **회전**: `Rotation / LastRotation / RotationDelta / ConstPitchTicks`
- **입력 플래그**: `Sprinting / Sneaking / InWater / Gliding / Crawling / UsingItem / TerrainCollision`
- **Grace**: `TeleportGrace / waterExitGrace / knockbackGrace / pendingKnockback`
- **전투**: `LastSwingTick / ClickTimestamps / LastAttackTime / LastAttackTarget / LastAttackTick / LastAttackCount`
- **엔티티 테이블**: `entityPos / uniqueToRID` (단일 최신 위치만)
- **이펙트**: `activeEffects`
- **레이턴시**: `latency`
- **기타**: `SimulationFrame / LastSimulationFrame / InputMode / GameMode / posInitialised / inputTimestamps / Violations`

### 3.4 proxy layer 현재

**clientToServer** (`proxy/proxy.go:118-281`):
- `PlayerAuthInput` 파싱 → 입력 플래그 추출 → 세션 sticky state 업데이트 → `ac.OnInput()` 호출
- `Animate/LevelSoundEvent` swing 이벤트
- `InventoryTransaction` (UseItemOnEntity = attack, UseItem ClickBlock = 블록 설치)

**serverToClient** (`proxy/proxy.go:286-399`):
- `AddPlayer/AddActor/MovePlayer/MoveActorAbsolute/MoveActorDelta` → 엔티티 위치 단일 캐시 업데이트
- `RemoveActor` → 정리
- `SetPlayerGameType` → GameMode 업데이트
- `MobEffect` → 이펙트 추가/제거
- `SetActorMotion/MotionPredictionHints` → 넉백 grace 기록

### 3.5 기존 코드의 구조적 한계

1. **클라 위치를 그대로 신뢰**: `UpdatePosition(pos mgl32.Vec3, onGround bool)`이 클라 보고값을 직접 저장. "기대 위치" 개념이 없음.
2. **월드 무지**: 블록 데이터 0. `OnGround`는 클라 자가신고, Phase는 델타 크기로만 추정, Scaffold는 발밑 블록 확인 없이 angle 휴리스틱.
3. **Entity Rewind 부재**: `entityPos`가 최신 1개 위치만. Reach가 공격 tick의 위치와 현재 위치가 달라서 오탐/우회 양쪽 취약.
4. **ACK 부재**: 클라가 보낸 tick을 그대로 신뢰. 텔레포트·넉백 이벤트가 클라에 "언제 반영되는지" 모름 → 넉백 grace를 RTT로 대충 추정.
5. **Correction 수단 없음**: kick만 가능. 경미한 위반에도 극단적 제재만 가능.

이 5가지가 §4 목표 아키텍처에서 해결됩니다.

---

## 4. 목표 아키텍처

### 4.1 컴포넌트 다이어그램

```
                        ┌─────────── proxy/proxy.go ──────────┐
                        │                                      │
  client ─ RakNet ─▶   │   clientToServer  ─▶  ack.System     │  ─▶ PMMP (RakNet)
                        │        │                │             │
                        │        ▼                ▼             │
                        │   anticheat.Manager ◀── pending cbs   │
                        │        │                              │
                        │        ├─▶ world.Tracker  ◀────────┐  │
                        │        ├─▶ entity.Rewind          │  │
                        │        ├─▶ sim.Engine ──▶─────────┘  │  chunks / blocks
                        │        ├─▶ data.Player              │
                        │        ├─▶ check.Registry (42)      │
                        │        └─▶ mitigate.Policy          │
                        │                │                      │
                        │                ▼                      │
  client ◀ RakNet ─   │  serverToClient ─── chunk/block ──▶ world.Tracker
                        │       │                              │
                        └──────────────────────────────────────┘
                              (correction MovePlayer if needed)
```

### 4.2 5개 신규 서브시스템

| 서브시스템 | 책임 | 패키지 | 담당 AI |
|-----------|------|--------|---------|
| **world** | LevelChunk/SubChunk/UpdateBlock 파싱, Dragonfly 청크 저장, 블록 lookup/BBox 제공 | `anticheat/world` | **AI-W** |
| **sim** | Bedrock 물리 엔진 복제 (걷기/뛰기/점프/중력/충돌/얼음/점액/물/사다리/포션/엘리트라/…) | `anticheat/sim` | **AI-S** |
| **entity** | 엔티티별 tick 위치 링버퍼, 특정 tick으로 rewind 조회 | `anticheat/entity` | **AI-E** |
| **ack** | NetworkStackLatency 마커 발사·수신, tick-sync 콜백 큐 | `anticheat/ack` | **AI-A** |
| **mitigate** | 위반 감지 시 policy 분기, client/server correction 패킷 발송 | `anticheat/mitigate` | **AI-M** |

### 4.3 패키지 구조 (after)

```
Better-pm-AC/
├── main.go
├── go.mod / go.sum            (+ github.com/df-mc/dragonfly)
├── config/config.go           (확장)
├── proxy/
│   ├── proxy.go               (확장 — 청크/블록 훅, ack 주입)
│   └── session.go
└── anticheat/
    ├── anticheat.go           (Manager — 신 서브시스템 배선)
    ├── meta/
    │   ├── detection.go       (+ Policy())
    │   └── interfaces.go      (신규 — world/sim/entity/ack/mitigate 인터페이스)
    ├── data/
    │   └── player.go          (확장 — worldRef, simRef, entityRewindRef)
    ├── world/                 (신규 — AI-W)
    │   ├── tracker.go
    │   ├── chunk_parser.go
    │   ├── subchunk_parser.go
    │   └── block_update.go
    ├── sim/                   (신규 — AI-S)
    │   ├── engine.go
    │   ├── walk.go
    │   ├── jump.go
    │   ├── gravity.go
    │   ├── collision.go
    │   ├── friction.go        (ice, slime, soul sand, honey)
    │   ├── fluid.go           (water, lava)
    │   ├── climbable.go       (ladder, vine, scaffolding)
    │   ├── effects.go         (Speed, JumpBoost, SlowFall, Levitation)
    │   ├── elytra.go          (γ)
    │   ├── riptide.go         (γ)
    │   ├── wind_charge.go     (γ)
    │   ├── powder_snow.go     (γ)
    │   ├── cobweb.go          (γ)
    │   └── state.go           (SimState: Position/Velocity/OnGround/Flags)
    ├── entity/                (신규 — AI-E)
    │   ├── rewind.go
    │   └── ringbuffer.go
    ├── ack/                   (신규 — AI-A)
    │   ├── system.go
    │   └── marker.go
    ├── mitigate/              (신규 — AI-M)
    │   ├── policy.go
    │   ├── client_rubberband.go
    │   └── server_filter.go
    └── checks/                (AI-C — 재작성/신규)
        ├── movement/          (기존 대체 + Step, HighJump, Jesus, Spider, InvalidMove)
        ├── combat/            (기존 개조 + Reach/B, KillAura/D)
        ├── packet/            (기존 유지 + BadPacket/F/G)
        ├── world/             (신규 — Nuker, FastBreak, FastPlace, Tower, InvalidBreak)
        └── client/            (신규 — EditionFaker, ClientSpoof)
```

### 4.4 데이터 플로우 (PlayerAuthInput 1 tick)

```
1. client → proxy    : PlayerAuthInput{Tick=N, Pos=P, InputFlags=…}
2. proxy.clientToServer :
   a. ack.System.OnTick(N)                            (pending callbacks 실행)
   b. data.Player.UpdateInputs(flags)
   c. sim.Engine.Step(player, input) → SimState       (기대 위치·속도 계산)
   d. data.Player.SetSimState(SimState)
   e. check.Registry.RunMovementChecks(player) :
      - Phase/A  : diff(P, SimState.Pos) vs BBox 통과 여부
      - Speed/A  : |P.xz − SimState.Pos.xz| vs tolerance
      - Fly/A/B  : |P.y − SimState.Pos.y| vs tolerance
      - NoFall/B : client.OnGround vs SimState.OnGround
      - ...
   f. 위반 있으면 mitigate.Policy 분기 :
      - ServerFilter : 패킷의 Position을 SimState.Pos로 덮어쓰기 → 서버로 전달
      - ClientRubberband : 클라에 MovePlayer{SimState.Pos} 발송
      - Kick : Fail() → VL 초과 시 connection.Close
3. proxy.clientToServer : 패킷을 server로 forward (수정되었을 수 있음)
```

---

## 5. 서브시스템 상세

### 5.1 world — 월드 트래커 (AI-W 소유)

**책임**: 플레이어 시야 내 청크 상태를 서버 권위 데이터(LevelChunk 등)로부터 복원, 블록 lookup과 BBox를 sim에 제공.

#### 5.1.1 의존성 · 타입

```go
import (
    "github.com/df-mc/dragonfly/server/world"
    "github.com/df-mc/dragonfly/server/world/chunk"
    "github.com/df-mc/dragonfly/server/block/cube"
)
```

- `chunk.Chunk`: Dragonfly의 Bedrock 청크 구조.
- `world.Block`: 블록 상태.
- `cube.BBox`: 블록 충돌 박스.

#### 5.1.2 인터페이스 (§11.1 참조)

```go
type Tracker interface {
    // HandleLevelChunk 서버→클라 LevelChunk 패킷을 소비, 청크 등록.
    HandleLevelChunk(pk *packet.LevelChunk) error
    // HandleSubChunk 동상 SubChunk.
    HandleSubChunk(pk *packet.SubChunk) error
    // HandleBlockUpdate UpdateBlock/UpdateBlockSynced.
    HandleBlockUpdate(pos cube.Pos, rid uint32) error
    // Block 블록 조회 (없으면 기본 air).
    Block(pos cube.Pos) world.Block
    // BlockBBoxes 블록 충돌박스(여러 개 가능).
    BlockBBoxes(pos cube.Pos) []cube.BBox
    // ChunkLoaded 해당 청크가 이미 로드되었는지.
    ChunkLoaded(x, z int32) bool
    // Close 리소스 정리.
    Close() error
}
```

#### 5.1.3 파싱 전략

- **LevelChunk**: `chunk.NetworkDecode(payload, subChunkCount, oldFormat)` 호출. Dragonfly가 서브청크 버퍼·팔레트·border 블록 전부 처리.
- **SubChunk (1.18+)**: Dimension별로 서브청크 개별 전송. `chunk.DecodeSubChunk()`.
- **UpdateBlock / UpdateBlockSynced**: 단일 블록 교체. `tracker.HandleBlockUpdate(pos, rid)` 로 청크의 해당 슬롯만 갱신.
- **오버월드 높이**: 1.18 이후 -64 ~ 320 지원. Dragonfly가 자동 대응.

#### 5.1.4 캐시 정책

- **플레이어별** 청크 저장 (공유 없음). 이유: 각 플레이어가 본 월드 상태가 서버와 완전 일치 보장 어려움(청크 언로드 타이밍 차이). 안전성 > 메모리 절약.
- 플레이어 연결 해제 시 전체 해제.
- 청크 언로드 패킷(`NetworkChunkPublisher` range 밖)은 수신 시 해당 청크 제거.

#### 5.1.5 성능 고려

- 청크 저장은 `map[uint64]*chunk.Chunk` (키: `(x<<32)|uint32(z)`).
- 읽기 압도적 빈번 → `sync.RWMutex`.
- 메모리 예산: 청크당 ≈ 32KB, 시야 거리 8청크 기준 289청크 ≈ 9MB. 예산 초과 시 시야 반경 축소 고려.

### 5.2 sim — 물리 시뮬레이션 엔진 (AI-S 소유)

**책임**: 클라이언트의 이전 상태 + 이번 tick 입력을 바탕으로 "vanilla Bedrock에서 일어났어야 할 상태"를 계산.

#### 5.2.1 SimState 구조

```go
type State struct {
    Position     mgl32.Vec3
    Velocity     mgl32.Vec3
    OnGround     bool
    InLiquid     bool      // water or lava
    InCobweb     bool
    InPowderSnow bool
    OnClimbable  bool      // ladder/vine/scaffolding
    OnSlime      bool
    OnIce        bool      // regular/packed/blue
    OnHoney      bool
    OnSoulSand   bool
    OnScaffolding bool
    Gliding      bool
    Riptiding    bool
    // JumpCooldown, StepHeight, etc.
}
```

#### 5.2.2 Step() 함수 — 핵심 알고리즘

```go
// Step advances simulation one tick.
// Returns the predicted state after applying gravity, input forces, collision.
func (e *Engine) Step(prev State, input Input, world Tracker) State {
    cur := prev
    // 1. Apply input (WASD → horizontal acceleration, scaled by speed modifiers)
    cur = applyInputForces(cur, input)
    // 2. Apply potion effects (Speed, JumpBoost, SlowFall, Levitation)
    cur = applyEffects(cur, input.Effects)
    // 3. Apply gravity (unless in liquid / gliding / levitation)
    cur = applyGravity(cur)
    // 4. Apply friction / drag (surface-dependent)
    cur = applyFriction(cur)
    // 5. Attempt movement with block collision
    cur = stepWithCollision(cur, world)
    // 6. Check on-ground / climbable / fluid / surface state
    cur = updateSurfaceFlags(cur, world)
    return cur
}
```

#### 5.2.3 물리 상수 (β 기준)

| 상수 | 값 | 출처 |
|------|-----|------|
| gravity | 0.08 b/tick² | Bedrock source |
| airDrag | 0.98 (수평/수직 공통) | |
| walkSpeed (base) | 0.1 | |
| sprintMultiplier | 1.3 | |
| sneakMultiplier | 0.3 | |
| swimMultiplier | 0.9 | |
| jumpVelocity (base) | 0.42 b/tick | |
| stepHeight | 0.6 b | |
| slimeBounce | -0.9 × yVel | |
| iceFrictionFactor | 0.98 → 0.991 | |
| honeyClimbDown | 0.05 b/tick | |
| cobwebSlow | × 0.15 | |
| powderSnowFall | × 0.4 | |

**근거 확인**: AI-S는 구현 전 Oomph `player/simulation/movement.go` 실제 코드 + Dragonfly `server/entity/player/movement.go`를 참고해 상수를 교차검증. 어긋나면 WORK-BOARD에 `Proposal` 추가.

#### 5.2.4 γ 전용 기능 (Phase 4)

- **Elytra**: `InputFlagStartGliding` 이후 활공 물리. pitch 기반 양력·저항, glide→freefall 전환.
- **Riptide**: 트라이던트 투척 후 rain/water에서 짧은 순간 강한 가속.
- **Wind Charge**: 착탄 후 velocity 추가.
- **Powder snow sink/climb**: leather boots가 있으면 climb, 없으면 sink.
- **Honey block slide**: 벽 타기 시 감속 하강.
- **Cobweb full slow**: 수평·수직 모두 × 0.15.
- **Scaffolding**: sneak으로 내려가기, 점프로 위로 올라가기.

### 5.3 entity — Entity Rewind (AI-E 소유)

**책임**: 엔티티별 위치 히스토리를 tick 단위 링버퍼로 저장, lag-compensated 조회 제공.

#### 5.3.1 구조

```go
type Snapshot struct {
    Tick     uint64
    Position mgl32.Vec3
    BBox     cube.BBox     // 공격 시 사용할 히트박스
    YawPitch mgl32.Vec2
}

type Rewind interface {
    // Record 엔티티의 현재 상태를 tick=N으로 저장.
    Record(rid uint64, tick uint64, pos mgl32.Vec3, bbox cube.BBox, rot mgl32.Vec2)
    // At 주어진 tick 시점의 스냅샷 반환. 없으면 가장 가까운 더 오래된 값.
    At(rid uint64, tick uint64) (Snapshot, bool)
    // Purge 엔티티 제거 시 히스토리 해제.
    Purge(rid uint64)
}
```

#### 5.3.2 윈도우 크기

- 기본 **40 tick = 2초 @ 20 TPS**.
- 플레이어 RTT가 500ms를 넘으면 auto-extend (최대 80 tick). Reach 검사는 `(now - RTT_in_ticks)` 스냅샷을 참조.

#### 5.3.3 메모리 예산

- 엔티티당 snapshot = 40 × 56 bytes ≈ 2.2KB. 시야 내 100엔티티 기준 220KB/player. 안전.

### 5.4 ack — Acknowledgement System (AI-A 소유)

**책임**: 서버가 클라에 보낸 "이벤트"(텔레포트·넉백·월드 수정 등)가 **언제 클라에서 실제로 처리되었는지** 확인. tick-synchronized 콜백.

#### 5.4.1 메커니즘: NetworkStackLatency 마커

Oomph와 동일 방식.

```
1. 서버가 중요 이벤트 X 발생 (예: 넉백 SetActorMotion) → 프록시가 가로챔
2. 프록시: 이 이벤트를 "pending queue에 쌓되, ack timestamp T를 할당"
3. 프록시가 클라에 NetworkStackLatency{ Timestamp: T, NeedsResponse: true } 발송
4. 프록시가 이벤트 X 패킷을 클라에 그대로 전달
5. 클라가 이벤트 X 처리 후, NetworkStackLatency 응답 {Timestamp: T} 송신
6. 프록시가 응답 수신 시점에 pending queue의 callback 실행 → "이제부터 X 결과를 검증 가능"
```

이렇게 하면 **"이 tick에서 knockback이 이미 반영되었는가?"** 가 확정 가능 → 기존 RTT-기반 추정 grace 대체.

#### 5.4.2 인터페이스

```go
type Callback func(tick uint64)

type System interface {
    // Dispatch 이벤트 발생 시 호출. cb는 클라가 확인응답 시 해당 tick과 함께 호출됨.
    Dispatch(cb Callback) (marker packet.Packet)
    // OnResponse 클라가 보낸 NetworkStackLatency 응답을 처리.
    OnResponse(timestamp int64, tick uint64)
    // PendingCount 대기 중인 콜백 개수 (디버그용).
    PendingCount() int
}
```

#### 5.4.3 용례

- **넉백 기록**: `SetActorMotion` 가로챈 뒤 `ack.Dispatch(func(tick){ player.MarkKnockbackApplied(tick) })` → Velocity/A가 이 tick부터 검증.
- **Rubberband correction**: 프록시가 클라에 `MovePlayer{correctedPos}` 보낸 뒤 `ack.Dispatch` → 확인 전까지 movement 검사 suspend.
- **월드 수정**: `UpdateBlock` 발송 후 `ack.Dispatch` → 블록 변경이 클라에 적용된 tick부터 collision 검사.

### 5.5 mitigate — Correction Policy (AI-M 소유)

**책임**: 위반 감지 시 체크의 `Policy()` 반환값에 따라 Kick/ClientRubberband/ServerFilter 실행.

#### 5.5.1 Policy enum

```go
type Policy int

const (
    PolicyKick             Policy = iota  // VL 초과 시 연결 종료
    PolicyClientRubberband                 // 클라에만 되돌리기 MovePlayer 발송
    PolicyServerFilter                     // 서버에는 수정된 위치 전달, 클라에는 되돌리기
    PolicyNone                             // 검사만 하고 제재 없음 (alert only)
)
```

#### 5.5.2 Dispatcher

```go
type Dispatcher interface {
    Apply(player *data.Player, detection Detection, violation *DetectionMetadata,
          packetUnderReview packet.Packet) (forwardPacket packet.Packet, kick bool)
}
```

- **Kick**: VL 초과 시 `true` 반환 + proxy가 connection 종료. VL 미초과면 원본 패킷 반환.
- **ClientRubberband**: 원본 패킷 반환 (서버로는 그대로), 동시에 클라에 correction `MovePlayer` 발송.
- **ServerFilter**: 원본 패킷의 `Position`을 `SimState.Position`으로 덮어쓴 새 패킷 반환.
- **None**: 원본 패킷 반환.

#### 5.5.3 충돌 방지

- 서로 다른 검사가 같은 tick에 다른 policy 발동 시 **우선순위**: Kick > ServerFilter > ClientRubberband > None.
- 예: Phase/A가 ServerFilter 요청하고 Fly/A가 Kick 요청하면 Kick 실행. ServerFilter는 패킷 덮어쓰기를 수행하되, Kick이 VL 초과라면 connection은 이미 닫히므로 무의미 → Kick이 승.

---

## 6. 체크 카탈로그 (44개)

### 6.1 카테고리별 요약

| 카테고리 | 유지 | 개조 | 신규 | 합계 |
|---------|------|------|------|------|
| Movement | 0 | 2 | 10 | 12 |
| Combat | 2 | 3 | 3 | 8 |
| Packet | 5 | 0 | 2 | 7 |
| World (신규 카테고리) | 0 | 1 | 6 | 7 |
| Client (신규 카테고리) | 0 | 0 | 3 | 3 |
| Misc | 3 | 0 | 2 | 5 |
| **합계** | **10** | **6** | **26** | **42** |

※ 기존 24개 중 일부는 대체되며 새 이름으로 편입. 여기서 "유지/개조/신규"는 최종 44개 관점.

### 6.2 전체 체크 목록

각 체크 필드:
- **Key**: `Type/SubType`
- **Inputs**: 검증에 필요한 신호
- **Logic**: 한 줄 요약
- **Policy**: Kick/ClientRubberband/ServerFilter/None
- **Config Key**: TOML 경로
- **FP Risk**: Low / Medium / High (오탐 가능성)

#### Movement (12)

| Key | Inputs | Logic | Policy | Config | FP |
|-----|--------|-------|--------|--------|----|
| Speed/A | SimState, ClientPos | 수평 속도 predicted vs actual diff > tolerance | ServerFilter | `speed` | L |
| Speed/B | SimState, ClientPos | Air 상태 한정 | ServerFilter | `speed_b` | L |
| Fly/A | SimState, ClientPos | Y 속도 predicted vs actual | ServerFilter | `fly` | L |
| Fly/B | SimState, HoverTicks | 중력 무시 hover | ServerFilter | `fly_b` | L |
| Fly/C | SimState.InLiquid, ClientPos | 유체 내 Fly | ServerFilter | `fly_c` | L |
| NoFall/A | FallDistance, HealthEvent | 낙하 데미지 기대 vs 실제 | Kick | `nofall` | L |
| NoFall/B | SimState.OnGround, Client.OnGround | OnGround spoof | ClientRubberband | `nofall_b` | L |
| Phase/A | BBox 충돌, position delta | 블록 BBox 관통 | ServerFilter | `phase` | M |
| Step/A | SimState.StepHeight, ΔY | 1블록 초과 point step | ServerFilter | `step` | M |
| HighJump/A | SimState.JumpVelocity, ΔY | 점프 높이 초과 | ServerFilter | `highjump` | L |
| Jesus/A | SimState.InLiquid, ClientPos.Y | 물 위에서 OnGround 상태 유지 | ServerFilter | `jesus` | M |
| Spider/A | BBox 충돌, Y 속도 | 수직 벽 상승 | ServerFilter | `spider` | M |
| InvalidMove/A | yaw/pitch snap | 순간 ±180° 회전 | Kick | `invalidmove` | L |
| NoSlow/A (개조) | UsingItem, SimState | 아이템 사용 중 기대 속도 초과 | Kick | `noslow` | L |
| Timer/A | PlayerAuthInput 레이트 | 1초당 20tick 초과 | Kick | `timer` | L |
| Velocity/A (개조) | 넉백 기대 velocity, ACK | Anti-KB | Kick | `velocity` | L |

※ 실제로는 13개지만 표 구조 상 Timer/Velocity/NoSlow를 Misc로 이동 고려. 합계는 위 카테고리 요약에 따라 조정.

#### Combat (8)

| Key | Inputs | Logic | Policy | Config | FP |
|-----|--------|-------|--------|--------|----|
| Reach/A (개조) | Rewind(attackTick), BBox | 공격 시점 타겟 BBox 거리 > 3.1 | Kick | `reach` | L |
| Reach/B (신규) | BBox raycast | 클라 눈→타겟 BBox 교차 검증 | Kick | `reach_b` | M |
| KillAura/A (유지) | CPS vs swing tick | swing 없이 hit | Kick | `killaura` | L |
| KillAura/B (개조) | 공격 시점 FOV (rewind) | 타겟이 시야 밖 | Kick | `killaura_b` | L |
| KillAura/C (유지) | 공격 tick 다중 타겟 | 한 tick에 2+ 엔티티 타격 | Kick | `killaura_c` | L |
| KillAura/D (신규) | 공격 직전 yaw snap | 회전 없이 타겟 전환 | Kick | `killaura_d` | M |
| AutoClicker/A (유지) | CPS | > 20 CPS | Kick | `autoclicker` | L |
| AutoClicker/B (유지) | 클릭 간격 stddev | < 5ms | Kick | `autoclicker_b` | M |
| AutoClicker/C (신규) | double-click 간격 분포 | 인간 디버그 패턴 이탈 | Kick | `autoclicker_c` | M |
| Aim/A (유지) | yaw 반올림 | 정수°로 스냅 | Kick | `aim` | L |
| Aim/B (유지) | const pitch | yaw 돌면서 pitch 0 | Kick | `aim_b` | M |

※ 실제 9개. 표 재조정 필요.

#### Packet (7)

| Key | Inputs | Logic | Policy | Config | FP |
|-----|--------|-------|--------|--------|----|
| BadPacket/A (유지) | tick 일치 | 서버 tick과 불일치 | Kick | `badpacket` | L |
| BadPacket/B (유지) | pitch ∈ [-90,90] | 범위 초과 | Kick | `badpacket_b` | L |
| BadPacket/C (유지) | sprint+sneak 동시 | 불가능 조합 | Kick | `badpacket_c` | L |
| BadPacket/D (유지) | NaN/Inf | Position NaN/Inf | Kick | `badpacket_d` | L |
| BadPacket/E (유지) | Start+Stop 동 tick | 모순 플래그 | Kick | `badpacket_e` | L |
| BadPacket/F (신규) | 필수 플래그 누락 | e.g. Sprinting w/o movement | Kick | `badpacket_f` | M |
| BadPacket/G (신규) | 패킷 순서 | Handshake 외 순서 위반 | Kick | `badpacket_g` | L |

#### World (7)

| Key | Inputs | Logic | Policy | Config | FP |
|-----|--------|-------|--------|--------|----|
| Scaffold/A (개조) | BlockPlace 각도, 발밑 블록 | 뒤를 보며 아래 설치 | Kick | `scaffold` | M |
| Nuker/A (신규) | BlockBreak 위치·tick | 동 tick 다중 블록 파괴 | Kick | `nuker` | L |
| Nuker/B (신규) | BlockBreak 범위 | 한 눈에 볼 수 없는 각도 | Kick | `nuker_b` | M |
| FastBreak/A (신규) | Break 시간 vs 블록 hardness | 예상보다 빠름 | Kick | `fastbreak` | M |
| FastPlace/A (신규) | BlockPlace CPS | > 기준 BPS | Kick | `fastplace` | L |
| Tower/A (신규) | 자기 아래 블록 설치 + 점프 | 연속 self-tower | Kick | `tower` | M |
| InvalidBreak/A (신규) | Break 타겟 각도·거리 | raycast 실패 | Kick | `invalidbreak` | M |

#### Client (3)

| Key | Inputs | Logic | Policy | Config | FP |
|-----|--------|-------|--------|--------|----|
| EditionFaker/A (신규) | Login 패킷 identity | Java UUID 패턴 | Kick | `editionfaker` | L |
| ClientSpoof/A (신규) | DeviceOS vs TitleID | Mismatch | Kick | `clientspoof` | L |
| Protocol/A (신규) | Protocol version | 허용 목록 외 | Kick | `protocol` | L |

### 6.3 체크별 상세 로직 (핵심만)

**Speed/A (대체)**:
```go
predicted := sim.State.Position.XZ()
actual    := player.ClientPos.XZ()
if actual.Sub(predicted).Len() > tolerance * (1 + latencyFactor) {
    detection.Fail()
    return mitigate.PolicyServerFilter
}
```

**Phase/A (대체)**:
- `sim.stepWithCollision` 이 이동 경로 상 BBox 충돌을 해결하고 `State.Position` 을 반환. 클라 Pos가 이를 관통했다면 위반.

**Reach/A (개조)**:
```go
attackTick := player.Tick
targetSnap := entity.Rewind.At(targetRID, attackTick - player.RTTTicks)
if !ok { return } // skip (no history)
eyePos := player.EyePos(attackTick)
dist := eyePos.Sub(targetSnap.BBox.Closest(eyePos)).Len()
if dist > 3.1 { detection.Fail() }
```

**Nuker/A (신규)**:
- 한 tick 내 `LevelEvent{LevelEventParticlesDestroyBlock}` 또는 `PlayerAction{BreakBlock}` 2건 이상 → VL.

**EditionFaker/A (신규)**:
- `Login.IdentityData.TitleID` 가 공식 Bedrock 앱 ID (`896928775` 등 화이트리스트) 에 포함되지 않으면 VL.

전체 상세 로직은 AI-C가 구현 전 `docs/check-specs/<key>.md` 로 분리 문서화 (Phase 3 초반에 Task).

---

## 7. 설정 스키마 변경

### 7.1 before (현재)

§1.3의 config.go 참조. 24개 체크의 `enabled/max_*/violations`.

### 7.2 after (목표)

신규 섹션과 확장된 체크 설정을 포함. 예시:

```toml
[proxy]
listen_addr = "0.0.0.0:19132"
remote_addr = "127.0.0.1:19133"

[anticheat]
# 전역 설정
log_violations   = true
log_level        = "info"

[anticheat.simulation]
# 물리 엔진 튜닝
tolerance_speed_xz  = 0.02   # 블록/tick, 속도 diff 허용
tolerance_fly_y     = 0.01
step_height         = 0.6
gravity_accel       = 0.08
air_drag            = 0.98

[anticheat.world]
# 청크 트래커
max_chunks_per_player = 400
chunk_cache_ttl_ticks = 6000  # 5분

[anticheat.entity]
rewind_window_ticks = 40
rewind_max_ticks    = 80

[anticheat.ack]
marker_timeout_ticks = 100   # 이 tick 안에 응답 없으면 드롭

# === 각 체크별 설정 (44개) ===
[anticheat.speed]
enabled    = true
fail_buffer = 3
max_buffer  = 10
violations  = 10
policy      = "server_filter"

[anticheat.phase]
enabled = true
violations = 3
policy = "server_filter"

# ... (44개 전체, 키 목록은 §6.2 Config 칼럼 참조)

[anticheat.editionfaker]
enabled = true
allowed_title_ids = [896928775, 2047319603, 1828326430]
violations = 1
policy = "kick"
```

### 7.3 마이그레이션

- 기존 `config.toml` 읽기 시 신 키가 없으면 **기본값 자동 삽입** + 저장. 사용자는 수동 작업 불필요.
- `policy` 필드는 기본값이 §6.2 표의 Policy 컬럼.

---

## 8. 파일별 수정 계획

### 8.1 신규 생성 (패키지 경계는 AGENT-PROTOCOL §2 참조)

| 파일 | 내용 | 담당 AI |
|------|------|---------|
| `anticheat/meta/interfaces.go` | world/sim/entity/ack/mitigate 인터페이스 정의 | AI-O |
| `anticheat/world/*.go` | §5.1 구현 | AI-W |
| `anticheat/sim/*.go` | §5.2 구현 | AI-S |
| `anticheat/entity/*.go` | §5.3 구현 | AI-E |
| `anticheat/ack/*.go` | §5.4 구현 | AI-A |
| `anticheat/mitigate/*.go` | §5.5 구현 | AI-M |
| `anticheat/checks/world/*.go` | 신규 카테고리 (Nuker, Tower 등) | AI-C |
| `anticheat/checks/client/*.go` | 신규 카테고리 (EditionFaker 등) | AI-C |
| `anticheat/checks/movement/{step,highjump,jesus,spider,invalidmove}.go` | 신규 movement | AI-C |
| `anticheat/checks/combat/{reach_b,killaura_d,autoclicker_c}.go` | 신규 combat | AI-C |
| `anticheat/checks/packet/{badpacketf,badpacketg}.go` | 신규 packet | AI-C |
| `docs/check-specs/*.md` | 각 체크 상세 스펙 | AI-C |
| `docs/forks-notes.md` | 포크 3개 조사 결과 | AI-O |

### 8.2 대규모 수정

| 파일 | 변경 |
|------|------|
| `anticheat/meta/detection.go` | `Policy()` 메서드 추가, `Correctable()` 추가 |
| `anticheat/data/player.go` | `SimState` 필드, `EntityRewindRef`, `WorldRef` 추가. 기존 `entityPos` 맵은 제거 (entity 패키지로 이관). `RecordKnockback` 시그니처 변경 (ack 연동) |
| `anticheat/anticheat.go` | Manager에 world/sim/entity/ack/mitigate 필드 추가, 배선 |
| `proxy/proxy.go` | serverToClient에 LevelChunk/SubChunk/UpdateBlock 훅 추가, ack.System 주입, correction 발송 경로 |
| `config/config.go` | §7.2 스키마 반영 |

### 8.3 수정 필요한 기존 체크

- `anticheat/checks/movement/{speed,speed_b,fly,fly_b,nofall,nofall_b,phase,scaffold}.go` — **대체** (sim 기반으로 재작성).
- `anticheat/checks/combat/{reach,killaura_b}.go` — **개조** (rewind 기반).
- `anticheat/checks/movement/{noslow,velocity}.go` — **개조** (sim 결과와 대조).
- 나머지는 `Policy()` 메서드만 추가.

### 8.4 삭제 대상

- 없음. 현재 체크는 다 유지 또는 교체되므로 파일 자체 삭제는 없음. (단, 기존 구현의 덩어리 함수를 재작성 시 이전 함수는 제거.)

---

## 9. 단계별 로드맵

### Phase 1 — Foundation (순차 · Orchestrator 단독 · 약 1주)

- **1.1**: 본 설계 MD 확정 (지금).
- **1.2**: `docs/forks-notes.md` — 포크 3개 조사.
- **1.3**: `anticheat/meta/interfaces.go` — 5개 인터페이스 stub 작성 (실구현은 Phase 2).
- **1.4**: `anticheat/meta/detection.go` 확장 (`Policy()`).
- **1.5**: `config/config.go` — 신 스키마 반영 + default.
- **1.6**: `go.mod` — `github.com/df-mc/dragonfly` 추가.
- **1.7**: 빈 패키지 디렉토리 + placeholder `doc.go` 생성 (`world/`, `sim/`, `entity/`, `ack/`, `mitigate/`, `checks/world/`, `checks/client/`).
- **1.8**: WORK-BOARD 작성 (Phase 2~5 태스크 등록).
- **1.9**: 빌드 녹색 확인 (`go build ./...` 통과).

### Phase 2 — Core Subsystems (병렬 · 약 2~3주)

Phase 1 완료 후 5개 AI 동시 착수:

- **AI-W** (world): 청크 파싱 · UpdateBlock · BBox 조회.
- **AI-S** (sim): β 물리 (엘리트라·릿프타이드 제외한 모든 것).
- **AI-E** (entity): 링버퍼 rewind.
- **AI-A** (ack): NetworkStackLatency 마커 시스템.
- **AI-M** (mitigate): 3가지 policy 실행기.

각 AI는 자기 패키지만 수정. 통합은 Phase 3에서.

### Phase 3 — Check Migration + 신규 (병렬 · 약 2~3주)

- **AI-C-1**: 기존 movement 체크 8개 sim 기반 재작성.
- **AI-C-2**: 기존 combat 체크 2개 rewind 기반 개조, 신규 combat 3개 추가.
- **AI-C-3**: 기존 packet 체크 5개 유지 + 신규 2개 (BadPacket/F,G).
- **AI-C-4**: 신규 world 카테고리 6개 (Nuker/A,B, FastBreak, FastPlace, Tower, InvalidBreak).
- **AI-C-5**: 신규 client 카테고리 3개 (EditionFaker, ClientSpoof, Protocol).

### Phase 4 — γ Physics 확장 (병렬 · 약 2~3주)

AI-S가 Phase 2 마친 뒤 γ 특수 물리 추가:
- 엘리트라 / 릿프타이드 / 윈드차지 / 가루눈 / 꿀 블록 / 거미줄 / 스캐폴딩 블록 / 슬라임 바운스 체인.

각 기능은 독립 Task로 WORK-BOARD에 등록 → 여러 AI 동시 가능.

### Phase 5 — Integration + Release (순차 · 약 1~2주)

- **5.1**: 모든 서브시스템을 `anticheat.Manager`에 배선.
- **5.2**: `proxy/proxy.go` 훅 추가 완료.
- **5.3**: 통합 테스트 (실제 PMMP 서버 + Bedrock 클라로 주요 체크 골든패스 검증).
- **5.4**: 성능 프로파일링 (목표 §1.4).
- **5.5**: **β 릴리스** (γ 기능 disabled 상태).
- **5.6**: Phase 4 완료 후 γ 기능 enable → **γ 릴리스**.

### 전체 예상 타임라인 (5 AI 병렬 전제)

| Phase | 기간 | 누적 | 비고 |
|-------|------|------|------|
| 1 | 1일 | 1일 | AI-O 단독. **완료 (2026-04-19)** |
| 2 | 2일 | 3일 | 5 AI (W/S/E/A/M) 병렬 |
| 3 | 3일 | 6일 | 5 서브트랙 (C1~C5) 병렬. Phase 4와 병행 가능 |
| 4 | 2일 | 6일 | AI-S 단독. Phase 3와 병행 |
| 5a | 2일 | 8일 | 통합 · 테스트 · 성능 검증 |
| 5b | 2일 | 10일 | β 릴리스 (γ 기능 disabled) |
| 5c (γ) | 4일 | 14일 | Phase 4 결과 enable → γ 릴리스 |

**β 출시 = Phase 1 + 2 + 3 + 5a + 5b = 약 10일 (2026-04-29 경)**.
**γ 출시 = β + 5c = 약 14일 (2026-05-03 경)**.

참고: 기간은 "AI 에이전트 연속 작업 시 wall-clock 기준". 사람이 직접 하면 원래 7.5주~10주 수준. 병렬 AI 전제가 무너지면 ETA 재협상.

---

## 10. 코딩 규약

### 10.1 패키지 네이밍 · Import 순서

- 패키지명: 한 단어, 모두 소문자 (`world`, `sim`, `entity`, `ack`, `mitigate`).
- Import 블록: stdlib → 3rd party → project internal (gofmt 기본).

### 10.2 타입 · 함수 네이밍

- **Exported**: CamelCase. `Tracker`, `Engine`, `Rewind`, `System`, `Dispatcher`.
- **Unexported**: lowerCamelCase.
- **Receiver**: 짧은 약자 (`t *tracker`, `e *engine`).
- **Interface**: 단일 메서드면 `-er` 접미사 (`Stepper`), 다중이면 명사 (`Tracker`).

### 10.3 주석 정책 (CLAUDE.md 연동)

- **WHY만 써라.** "이 함수는 X를 한다"는 금지.
- 비자명 제약·버그 우회·물리 상수 근거는 필수.
- 단일 함수 doc comment는 한 줄.
- 다중 줄 block comment 금지 — 패키지 doc.go 외.

### 10.4 오류 처리

- `error` 반환 타입 유지, wrap 시 `fmt.Errorf("context: %w", err)`.
- panic 금지 (단, `init()` 패닉 가능 — 프로그래밍 오류).
- 체크 로직 내부에서 recover 금지.

### 10.5 동시성

- `sync.RWMutex` 기본. `Mutex`는 writer 압도적 빈번 시만.
- 채널은 "소유자 = sender close" 규칙 엄수.
- goroutine 시작 시 컨텍스트 전달 + cancel 청소 보장.

### 10.6 테스트

- `*_test.go` in same package.
- 물리 시뮬 테스트는 `testdata/` 에 실측 Bedrock 세션 로그(녹화) 비교.
- 각 체크는 최소 3 케이스: 합법 입력, 명확한 치트, 경계치.

### 10.7 커밋 메시지 (AGENT-PROTOCOL §3 연동)

- 형식: `[phase.task] <verb> <subject>`
- 예: `[2.5] add sim engine core Step()`, `[3.12] migrate Fly/A to sim-based diff`.
- Co-Authored-By 블록은 Claude 명시.

---

## 11. 인터페이스 계약 (AI 간 공유)

이 섹션의 인터페이스는 **Phase 1에서 AI-O가 `anticheat/meta/interfaces.go`로 구현**합니다. Phase 2 이후 어떤 AI도 임의 수정 금지. 변경 필요 시 `Proposal` 필수.

### 11.1 world.Tracker

```go
package meta

import (
    "github.com/df-mc/dragonfly/server/block/cube"
    "github.com/df-mc/dragonfly/server/world"
    "github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type WorldTracker interface {
    HandleLevelChunk(pk *packet.LevelChunk) error
    HandleSubChunk(pk *packet.SubChunk) error
    HandleBlockUpdate(pos cube.Pos, rid uint32) error
    Block(pos cube.Pos) world.Block
    BlockBBoxes(pos cube.Pos) []cube.BBox
    ChunkLoaded(x, z int32) bool
    Close() error
}
```

### 11.2 sim.Engine

```go
package meta

import "github.com/go-gl/mathgl/mgl32"

type SimInput struct {
    Forward, Strafe float32
    Jumping         bool
    Sprinting       bool
    Sneaking        bool
    Swimming        bool
    UsingItem       bool
    GlideStart      bool
    GlideStop       bool
    Riptiding       bool
    Effects         map[int32]int32   // effectID → amplifier
}

type SimState struct {
    Position      mgl32.Vec3
    Velocity      mgl32.Vec3
    OnGround      bool
    InLiquid      bool
    InCobweb      bool
    InPowderSnow  bool
    OnClimbable   bool
    OnSlime       bool
    OnIce         bool
    OnHoney       bool
    OnSoulSand    bool
    OnScaffolding bool
    Gliding       bool
    Riptiding     bool
}

type SimEngine interface {
    Step(prev SimState, input SimInput, world WorldTracker) SimState
}
```

### 11.3 entity.Rewind

```go
package meta

import (
    "github.com/go-gl/mathgl/mgl32"
    "github.com/df-mc/dragonfly/server/block/cube"
)

type EntitySnapshot struct {
    Tick     uint64
    Position mgl32.Vec3
    BBox     cube.BBox
    Rotation mgl32.Vec2
}

type EntityRewind interface {
    Record(rid uint64, tick uint64, pos mgl32.Vec3, bbox cube.BBox, rot mgl32.Vec2)
    At(rid uint64, tick uint64) (EntitySnapshot, bool)
    Purge(rid uint64)
}
```

### 11.4 ack.System

```go
package meta

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

type AckCallback func(tick uint64)

type AckSystem interface {
    Dispatch(cb AckCallback) packet.Packet
    OnResponse(timestamp int64, tick uint64)
    PendingCount() int
}
```

### 11.5 mitigate.Dispatcher

```go
package meta

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

type MitigatePolicy int

const (
    PolicyNone MitigatePolicy = iota
    PolicyClientRubberband
    PolicyServerFilter
    PolicyKick
)

type MitigateDispatcher interface {
    Apply(playerUUID string, d Detection, meta *DetectionMetadata,
          original packet.Packet) (forwarded packet.Packet, kick bool)
}
```

### 11.6 Detection (확장)

```go
type Detection interface {
    Type() string
    SubType() string
    Description() string
    Punishable() bool
    DefaultMetadata() *DetectionMetadata
    Policy() MitigatePolicy    // 신규
}
```

---

## 12. 용어집

| 용어 | 정의 |
|------|------|
| **MiTM** | Man-in-the-Middle. 프록시가 클라-서버 중간에 앉아 패킷을 가로채는 구조. |
| **Tick** | Minecraft 시뮬레이션 단위. Bedrock 기본 20 TPS = 50ms/tick. |
| **Simulation / Sim** | 프록시 안에서 클라의 물리를 서버 권위로 재현. "기대 상태" 계산. |
| **Rewind** | Entity Rewind. 엔티티의 과거 위치를 tick별 조회. |
| **ACK (Acknowledgement)** | 이벤트가 클라에서 반영된 시점 확인 메커니즘. |
| **Rubberband** | 클라 위치를 서버 기준으로 강제로 되돌리는 correction. |
| **Buffer/FailBuffer/MaxBuffer** | Oomph식 VL 누적 시스템. 연속 위반만 실제 VL 추가. |
| **Policy** | 위반 감지 시의 제재 방식 (Kick/ClientRubberband/ServerFilter/None). |
| **Orchestrator / AI-O** | 다중 AI 협업에서 인터페이스 관리·Phase 순차 Task 담당. |
| **SSoT** | Single Source of Truth. 이 설계 문서가 SSoT. |

---

## 13. 관측성 (Observability)

운영 중 어떤 체크가 어떻게 동작했는지 역추적 가능해야 함. AI-M이 Phase 3에서 구현.

### 13.1 이벤트 로그

모든 Detection flag는 structured log (slog) 한 줄:

```
level=warn msg="ac.flag" player=<uuid> check=Speed/A vl=3 buffer=5 policy=server_filter tick=12345 pos="x,y,z" reason="dx=0.58 > tol 0.02"
```

필드 고정: `player`, `check`, `vl`, `buffer`, `policy`, `tick`, `pos`, `reason`. 로그 파서가 이 포맷을 전제로 만들어짐.

### 13.2 메트릭

AI-M이 아래 카운터 노출 (Prometheus `/metrics` 엔드포인트, Phase 5b):

| 메트릭 | 타입 | 라벨 |
|--------|------|------|
| `pmac_flags_total` | Counter | `check`, `policy`, `result` (logged/rubberband/kick) |
| `pmac_players_active` | Gauge | - |
| `pmac_tick_duration_seconds` | Histogram | `phase` (recv/sim/check/dispatch) |
| `pmac_chunk_cache_size_bytes` | Gauge | `player` |
| `pmac_ack_pending` | Gauge | - |
| `pmac_rewind_buffer_entries` | Gauge | - |

### 13.3 디버그 덤프

`Kick` 발생 직전 5초 (100 tick) 입력·상태 스냅샷을 `dumps/<uuid>-<timestamp>.json`에 저장 (옵션, 기본 off). 오탐 분석 필수.

---

## 14. 성능 예산 (Memory / CPU)

100 CCU를 **서버 한 대 · 4 vCPU · 4GB RAM** 에서 처리. Phase 5a 벤치에서 이 선을 지키지 못하면 블로커로 간주.

### 14.1 플레이어당 메모리 상한

| 항목 | 상한 | 근거 |
|------|------|------|
| Chunk cache | 400 chunk × ~40KB = **~16MB** | `config.world.max_chunks_per_player` |
| Entity rewind | 엔티티 50 × 80 tick × 96B = **~400KB** | `config.entity.rewind_max_ticks` |
| Sim state / input ring | **~64KB** | 200 tick history |
| Detection metadata | 44 × 128B = **~6KB** | §6 카탈로그 전체 |
| 합계 | **~17MB / player** | |

100 player × 17MB = **1.7GB**. 4GB 중 2.3GB 여유. OS·Go 런타임 포함해도 안전.

### 14.2 CPU 예산 (per tick)

서버는 20 TPS (50ms/tick). 100 player의 한 tick 처리에 **p99 < 20ms, p50 < 5ms** 목표.

| 단계 | p50 예산 | p99 예산 |
|------|---------|---------|
| 패킷 파싱 | 0.5ms | 2ms |
| 시뮬레이션 (`sim.Step`) | 1.5ms | 6ms |
| 체크 dispatch (44개) | 1.5ms | 6ms |
| Mitigate + 출력 | 1ms | 4ms |
| 여유 | 0.5ms | 2ms |
| 합계 | **5ms** | **20ms** |

초과 시 AI-S는 sim 핫패스 최적화 (GetNearbyBBoxes 등), AI-W는 chunk 인덱스 구조 재검토.

### 14.3 벤치 시나리오 (Phase 5a `5.4`)

1. 봇 100대 동시 접속 → `/metrics` 에서 p99 tick 20ms 이하 유지 10분.
2. Speed 치트 봇 10대 + 합법 봇 90대 → 치트만 정확히 flagged, 합법은 0 violation.
3. 1시간 지속 세션 → RSS 메모리 단조증가 없음 (leak 체크).

---

**문서 끝.** 다음 단계는 **[WORK-BOARD.md](./2026-04-19-work-board.md)** 에서 현재 `pending` Task를 확인하세요.
