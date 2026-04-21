# Better-pm-AC

고성능 Minecraft Bedrock Edition 안티치트 프록시 — Go + [Gophertunnel](https://github.com/Sandertv/gophertunnel) 기반의 MiTM(Man-in-the-Middle) 방식으로 동작합니다. PMMP 서버 앞단에 붙여, 치트 패킷을 서버가 보기 전에 탐지·차단합니다.

```
플레이어 → [Better-pm-AC 프록시] → (RakNet/UDP) → [PMMP 서버]
```

현재 버전: **v0.10.0-beta**. β 릴리스는 17 종 체크 + 시뮬레이터 + 월드 트래커 + rewind 버퍼까지 포함합니다. γ 스펙(Phase 4 physics / world-block / client fingerprint checks)은 `docs/check-specs/` 에 문서화되어 있고 구현 진행 중입니다 (`CHANGELOG.md` 참조).

---

## 아키텍처 개요

| 컴포넌트 | 역할 |
|---|---|
| `proxy/` | MiTM 프록시. RakNet 세션 두 개(클→프록시, 프록시→서버)를 페어링하고, 패킷을 `anticheat.Manager` 로 포워딩 |
| `anticheat/` | 체크 조율 매니저. 플레이어 등록/해제, 검사 슬라이스, 위반 집계 |
| `anticheat/sim/` | 결정론적 물리 시뮬레이터. 중력·마찰·충돌·점프 부스트·액체 drag·이펙트 모디파이어 |
| `anticheat/world/` | 플레이어별 청크 캐시 + 블록 업데이트 반영. Reach / 블록 체크가 조회 |
| `anticheat/entity/` | 틱 기반 엔티티 위치 rewind 링버퍼 (ping-compensated reach) |
| `anticheat/ack/` | 틱 키 기반 pending-action 큐. 시뮬레이터 결정이 클라 확인 후에만 커밋되도록 |
| `anticheat/mitigate/` | 위반 시 동작 라우팅 (`none` / `client_rubberband` / `server_filter` / `kick`) + `expvar` 메트릭 |
| `anticheat/checks/` | 개별 체크 구현 (movement / combat / packet / world / client) |

---

## 탑재 체크 (β 스코프 17 종)

| 카테고리 | 체크 | 설명 |
|---|---|---|
| Movement | **Speed/A** | 수평 이동 속도 상한 초과 (기본 0.4 b/tick) |
| Movement | **Fly/A** | 공중에서 호버링·슬로우 낙하 감지 (GravViolTicks 누적) |
| Movement | **NoFall/A** | 낙하 거리 vs. 받은 데미지 불일치 — vanilla 공식 대비 |
| Movement | **Phase/A** | 텔레포트 없이 벽을 관통한 비정상 Δpos 감지 |
| Movement | **NoSlow/A** | 먹기/활/방패 사용 중 이동 속도 제한 우회 감지 |
| Movement | **Velocity/A** | 서버가 준 knockback 을 클라가 무시(Anti-KB) 감지 |
| Combat | **Reach/A** | 공격 범위 3.1 b 초과 (1-block ping 보정 포함) |
| Combat | **AutoClicker/A** | CPS 16 초과 |
| Combat | **Aim/A** | round-match 3e-5 / 1e-3 idle-gate (mouse 전용) |
| Combat | **KillAura/A** | 팔 흔들기 없이 공격 (swing-less hit) |
| Combat | **KillAura/B** | 시야각 90° 밖 타깃 공격 |
| Combat | **KillAura/C** | 한 틱에 여러 타깃 동시 공격 |
| Packet | **BadPacket/A** | 잘못된 `PlayerAuthInput.Tick` 순서 |
| Packet | **BadPacket/B** | Pitch 범위 (±90°) 초과 |
| Packet | **BadPacket/C** | Sprint + Sneak 동시 플래그 (vanilla 불가능) |
| Packet | **BadPacket/D** | NaN/Infinity 좌표 (poison packet) |
| Packet | **BadPacket/E** | Start* / Stop* 플래그 동시 세팅 (모순 bitset) |

체크 설정은 `config.toml` 의 `[anticheat.<name>]` 섹션으로 항목별 토글·임계값·정책을 바꿀 수 있습니다. 정책 옵션은 `kick` / `client_rubberband` / `server_filter` / `none` — 자세한 내용은 `docs/ops-runbook.md` 참조.

---

## 완화 정책 (Mitigation)

각 체크는 플래그 발생 시 `Policy()` 가 지정한 경로로 라우팅됩니다:

- `kick` — 누적 위반이 `max_violations` 를 넘으면 `KickFunc` 이 연결을 끊습니다.
- `client_rubberband` — `MovePlayer` + `MoveModeReset` 으로 서버 권위 위치로 스냅. 치트 움직임을 되돌리되 세션은 유지.
- `server_filter` — β 에서는 hook 이 비어 있어 `dry_run_suppressed` 카운터만 올라갑니다. γ 에서 패킷 파이프라인 재설계와 함께 실제 rewrite 로 동작합니다.
- `none` — 로깅만 하고 통과 (false-positive 의심 중인 체크를 관측 모드로 돌릴 때).

nil hook 경로는 모두 `better_pm_ac_dry_run_suppressed_total` 에 집계되어, 드라이런 모드에서도 감지 신호를 잃지 않습니다.

---

## 메트릭 (expvar)

`net/http/pprof` 가 import 된 기본 설정에서 `/debug/vars` 에 다음 카운터가 자동 노출됩니다:

| 카운터 | 의미 |
|---|---|
| `better_pm_ac_violations_total` | `Policy() != none` 인 모든 Apply 호출 |
| `better_pm_ac_kicks_total` | MaxViolations 를 넘겨 실제 KickFunc 이 호출된 횟수 |
| `better_pm_ac_rubberbands_total` | RubberbandFunc 이 호출된 횟수 |
| `better_pm_ac_filtered_packets_total` | ServerFilterFunc 이 호출된 횟수 |
| `better_pm_ac_dry_run_suppressed_total` | nil hook 때문에 실집행 없이 로깅만 된 결정 수 |

운영 매뉴얼·알림 threshold·Grafana 대시보드 샘플은 `docs/ops-runbook.md` 에 있습니다.

---

## 프로젝트 구조

```
Better-pm-AC/
├── main.go                         # 진입점
├── config/
│   └── config.go                   # TOML 설정 (~40개 체크 필드)
├── proxy/
│   ├── proxy.go                    # MiTM 코어 + RubberbandFunc 배선
│   └── session.go                  # 세션 페어
├── anticheat/
│   ├── anticheat.go                # Manager (OnInput / OnAttack / OnMove / OnBlockPlace)
│   ├── integration_test.go         # 시나리오 통합 테스트 (legit 3 + cheat 5)
│   ├── data/player.go              # 세션별 상태 (rotation / input / entity pos / effects)
│   ├── meta/                       # Detection / DetectionMetadata / MitigatePolicy 계약
│   ├── sim/                        # 결정론적 물리 엔진
│   ├── world/                      # 청크 캐시 + 블록 업데이트 반영
│   ├── entity/                     # rewind 링버퍼
│   ├── ack/                        # tick-keyed pending-action 큐
│   ├── mitigate/                   # 정책 dispatcher + expvar 메트릭
│   └── checks/
│       ├── movement/               # Speed / Fly / NoFall / NoSlow / Phase / Velocity / Timer / Scaffold
│       ├── combat/                 # Reach / KillAura(A/B/C) / AutoClicker / Aim
│       ├── packet/                 # BadPacket A~E
│       ├── world/                  # (γ 구현 중) FastPlace / Nuker / InvalidBreak ...
│       └── client/                 # (γ 구현 중) Protocol / EditionFaker / ClientSpoof
├── bench/loadtest_test.go          # 100 CCU / OnAttack / 세션 churn 벤치
├── docs/
│   ├── check-specs/                # 체크별 스펙 문서
│   ├── plans/                      # 로드맵 & work-board
│   ├── ops-runbook.md              # 운영 매뉴얼
│   └── physics-constants.md        # vanilla 물리 상수 참조
├── .github/workflows/ci.yml        # Linux + Windows 매트릭스, golangci-lint, nightly bench
├── .golangci.yml                   # 린터 설정
├── CHANGELOG.md                    # 릴리스 노트
└── go.mod / go.sum
```

---

## 빠른 시작

### 요구사항

- Go 1.24 이상 (CI 와 정확히 동일한 `go.mod` 요구 버전)
- PMMP 서버 (PocketMine-MP 5.x)

### 빌드

```bash
git clone https://github.com/boredape874/Better-pm-AC.git
cd Better-pm-AC
go build -o better-pm-ac .
```

### 실행

```bash
./better-pm-ac
```

처음 실행 시 기본값으로 `config.toml` 이 자동 생성됩니다. 이후 체크 임계값·정책을 그 파일에서 조정하세요.

### 포트 설정

| 구분 | 기본값 |
|---|---|
| 프록시 listen | `0.0.0.0:19132` |
| PMMP 서버 | `127.0.0.1:19133` |

PMMP 리슨 포트를 `19133` 으로 변경하거나, `config.toml` 의 `proxy.remote_addr` 값을 수정하세요.

### 테스트 / 벤치

```bash
go test ./...                                      # 유닛 + 시나리오
go test -bench=. -benchtime=1s -run=^$ ./bench/... # 성능 벤치
go vet ./...
```

CI(`.github/workflows/ci.yml`)는 동일한 명령을 Linux + Windows 매트릭스에서 돌리고, 별도 `golangci-lint` job 에서 13 종 린터(errcheck / staticcheck / gosec ...)를 실행합니다.

---

## 설정 예시 (`config.toml`)

```toml
[proxy]
listen_addr = "0.0.0.0:19132"
remote_addr = "127.0.0.1:19133"

[anticheat]
log_violations = true
log_level      = "info"

[anticheat.speed]
enabled    = true
policy     = "server_filter"   # server_filter / client_rubberband / kick / none
max_speed  = 0.4               # blocks/tick
violations = 10

[anticheat.reach]
enabled    = true
policy     = "kick"
max_reach  = 3.1
violations = 7

[anticheat.autoclicker]
enabled    = true
policy     = "kick"
max_cps    = 20
violations = 20

[anticheat.badpacket]
enabled    = true
policy     = "kick"
violations = 1                 # 단발로 킥 (명백한 프로토콜 위반)

# ... 전체 ~40개 체크 섹션. 처음 기동 시 전부 기본값으로 자동 생성.
```

모든 체크에 공통으로 있는 필드:

- `enabled` — 소프트 토글
- `policy` — 위반 처리 방식
- `violations` — 누적 위반 임계값 (kick 정책일 때만 의미)

검사별 세부 파라미터(`max_speed`, `max_reach`, `max_cps`, `max_bps` ...)는 각 스펙(`docs/check-specs/`)에 근거 숫자가 문서화되어 있습니다.

---

## 개발 현황

- **β (v0.10.0-beta)** — Phase 1 foundation + Phase 2 시뮬레이터/월드/rewind/ack + Phase 3 체크 17종 + Phase 5 릴리스 인프라 (CI, 메트릭, 통합/벤치 하네스, 런북). `CHANGELOG.md` 의 "Known limitations" 섹션이 γ 에 남은 항목을 명시합니다.
- **γ (진행 중)** — Phase 4 물리 확장(Elytra/Riptide/Wind charge/Powder snow/Honey/Cobweb/Scaffolding/Slime) · 월드 의존 블록 체크(Nuker/FastBreak/FastPlace/Tower/InvalidBreak) · 로그인 fingerprint 체크(Protocol/EditionFaker/ClientSpoof) · ServerFilter hook 배선 · per-check 메트릭 breakdown.

로드맵 세부: `docs/plans/2026-04-19-work-board.md`.

---

## 참고 프로젝트

- [Gophertunnel](https://github.com/Sandertv/gophertunnel) — Bedrock 프로토콜 구현체
- [Oomph](https://github.com/oomph-ac/oomph) — Go 기반 Bedrock 안티치트 프록시 (아키텍처 참고)
- [GrimAC](https://github.com/GrimAnticheat/Grim) — Java 기반 안티치트 (시뮬레이터 구조 참고)

---

## 라이선스

[MIT](LICENSE)
