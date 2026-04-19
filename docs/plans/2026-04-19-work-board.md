# Work Board — Better-pm-AC 전면 재설계

> **문서 위치**: `docs/plans/2026-04-19-work-board.md`
> **역할**: 실시간 Task 상태 추적. 모든 AI가 claim/complete 시 업데이트.
> **읽기 전**: [design.md](./2026-04-19-anticheat-overhaul-design.md) + [AGENT-PROTOCOL.md](./2026-04-19-agent-protocol.md) 먼저.

---

## 사용법 요약

1. 본인 역할(AI-?) 섹션에서 `Status: pending` 이고 `Depends on:` 이 모두 `done`인 Task 선택.
2. `Status: claimed-by-<역할>-<ISO시각>` 으로 변경 후 커밋 `[board] claim <id>`.
3. 작업.
4. 완료 시 `Status: done` + `Completed: <역할> <ISO시각>` + `Commits: <sha 목록>` 추가, 커밋 `[board] complete <id>`.
5. 블록되면 `Status: blocked` + `Blocker:` 필드.

**Status 값**: `pending` | `claimed-by-<...>` | `in-progress` | `blocked` | `done` | `canceled`.

---

## 현재 상황 (live)

| 항목 | 값 |
|------|-----|
| Phase 진행 중 | **Phase 2 진행 중** (Entity/Ack/Mitigate 기본 완료. World/Sim 남음) |
| 전체 진도 | 16 / ~75 Tasks done (Phase 1 전체 + 2.E.1–2, 2.A.1–2, 2.M.1–2) |
| β 마일스톤 ETA | **+10일 (2026-04-29 경)** — 5 AI 병렬 전제 |
| γ 마일스톤 ETA | **+14일 (2026-05-03 경)** |
| 현재 활성 AI | AI-O 겸임 (단일 세션, 경량 Task부터 순차 처리) |
| Urgent 이슈 | 없음 |

---

## Proposals

(설계 변경 제안은 여기에. AGENT-PROTOCOL §6 참조)

*현재 없음.*

---

## Urgent

(긴급 블로커)

*현재 없음.*

---

# Phase 1 — Foundation (순차, AI-O 단독)

Phase 1 완료 전까지 **다른 AI는 Task claim 금지**. 인터페이스와 빈 패키지 스켈레톤이 있어야 병렬 작업이 가능합니다.

### Task 1.1 — 설계 문서 작성
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `docs/plans/2026-04-19-anticheat-overhaul-design.md`
- Commits: (초기 작성 커밋)
- Acceptance: 설계 문서 §0-§12 전부 존재.

### Task 1.2 — 다중 AI 협업 문서 작성
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `docs/plans/2026-04-19-agent-protocol.md`, `docs/plans/2026-04-19-work-board.md`
- Commits: (초기 작성 커밋)
- Acceptance: AGENT-PROTOCOL + WORK-BOARD 존재.

### Task 1.3 — 포크 3개 조사 및 차이점 정리
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `docs/forks-notes.md`
- Notes: TrixNEW/wall-collissions + basicengine92/feat/lag-comp-cutoff + refactor/movement-sim + feat/scaffold-dtc 채택 권장.

### Task 1.4 — 인터페이스 stub 작성
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `anticheat/meta/interfaces.go`
- Notes: WorldTracker / SimEngine / EntityRewind / AckSystem / MitigateDispatcher + SimInput/SimState/EntitySnapshot/MitigatePolicy stubs.

### Task 1.5 — Detection 확장 (Policy 추가)
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `anticheat/meta/detection.go` + 24 existing check files.
- Notes: 모든 기존 체크에 `Policy() meta.MitigatePolicy { return meta.PolicyKick }` 일괄 추가.

### Task 1.6 — 패키지 스켈레톤 생성
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `anticheat/{world,sim,entity,ack,mitigate}/doc.go`, `anticheat/checks/{world,client}/doc.go`, `docs/check-specs/README.md`.

### Task 1.7 — Dragonfly 의존성 추가
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Notes: `github.com/df-mc/dragonfly v0.10.12` 추가. mathgl v1.2.0 / go-raknet v1.15.1 / x/mod v0.22 / x/text v0.23 / x/net v0.38 자동 업그레이드.

### Task 1.8 — 설정 스키마 확장
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Files: `config/config.go`
- Notes: Simulation/World/Entity/Ack 섹션 + 20개 신규 체크 config + LogViolations/LogLevel. Per-check `Policy` 필드는 Task 1.10에서 일괄 추가.

### Task 1.9 — 빌드 검증
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-19
- Result: `go build ./...` / `go vet ./...` / `go test ./...` 전부 녹색.

### Task 1.10 — Per-check Policy 필드 배선
- Status: **done**
- Owner: AI-O
- Completed: 2026-04-20
- Commits: 141afef
- Files: `config/config.go`, `anticheat/checks/**/*.go`, `anticheat/meta/interfaces.go`
- Notes: 44개 체크 config에 `Policy string` + 기본값, `meta.ParsePolicy` 헬퍼, 24개 기존 체크 `Policy()` 를 `c.cfg.Policy` 파싱으로 변경. build/vet/test 녹색.

---

# Phase 2 — Core Subsystems (병렬 · AI-W/S/E/A/M)

**Phase 1 완료 후 착수**. 5개 AI가 동시에 자기 패키지 작업.

## Phase 2 / AI-W — World Tracker

### Task 2.W.0 — testdata 캡처 계획
- Status: **pending**
- Owner: AI-W
- Depends on: 1.10
- Files: `testdata/README.md`, `testdata/level_chunk_overworld.bin`, `testdata/subchunk.bin`
- Acceptance:
  - 실제 Bedrock 세션에서 `packet.LevelChunk` / `packet.SubChunk` / `packet.UpdateBlock` 각 1개 이상 캡처 후 저장.
  - 캡처 방법 README에 기록 (예: `proxy.go`에 임시 dump hook). 제3자 AI가 재현 가능하게.
  - 실패 시 Blocker 표시, AI-O에 에스컬레이트.
- **2.W.2 이후 모든 Task가 이 파일에 의존함.**

### Task 2.W.1 — Tracker 구조체 + 생성자
- Status: **pending**
- Owner: AI-W
- Depends on: 1.10
- Files: `anticheat/world/tracker.go`
- Acceptance:
  - `tracker` struct + `NewTracker(log *slog.Logger) *tracker` + `meta.WorldTracker` 인터페이스 만족.
  - 모든 메서드 기본 stub (compile 통과).

### Task 2.W.2 — LevelChunk 파싱
- Status: **pending**
- Owner: AI-W
- Depends on: 2.W.1
- Files: `anticheat/world/chunk_parser.go`
- Acceptance:
  - `HandleLevelChunk(*packet.LevelChunk)` 가 Dragonfly `chunk.NetworkDecode`로 파싱, 내부 맵에 저장.
  - 단위 테스트: 실제 게임 캡처된 `testdata/level_chunk_overworld.bin` 디코딩 성공.

### Task 2.W.3 — SubChunk 패킷 처리
- Status: **pending**
- Owner: AI-W
- Depends on: 2.W.2
- Files: `anticheat/world/subchunk_parser.go`
- Acceptance:
  - 1.18+ SubChunk 패킷 각 엔트리를 해당 청크의 서브청크에 병합.
  - `testdata/subchunk.bin` 디코딩 테스트.

### Task 2.W.4 — UpdateBlock 처리
- Status: **pending**
- Owner: AI-W
- Depends on: 2.W.1
- Files: `anticheat/world/block_update.go`
- Acceptance:
  - `HandleBlockUpdate(pos, rid)` 가 해당 청크의 블록 슬롯을 교체.
  - 청크 미로드 좌표는 로그 + 드롭.
  - 단위 테스트.

### Task 2.W.5 — 블록 조회 + BBox
- Status: **pending**
- Owner: AI-W
- Depends on: 2.W.2, 2.W.4
- Files: `anticheat/world/tracker.go` 확장
- Acceptance:
  - `Block(pos cube.Pos) world.Block` 구현.
  - `BlockBBoxes(pos cube.Pos) []cube.BBox` — Dragonfly `block.Model()` 재사용.
  - 테스트: stone/air/ladder/slab BBox 반환 검증.

### Task 2.W.6 — 청크 언로드
- Status: **pending**
- Owner: AI-W
- Depends on: 2.W.1
- Files: `anticheat/world/tracker.go`
- Acceptance:
  - `NetworkChunkPublisher` 패킷 수신 시 range 밖 청크 언로드.
  - 메모리 누수 테스트 (1시간 세션 시뮬레이션).

### Task 2.W.7 — proxy.serverToClient 훅
- Status: **pending**
- Owner: AI-O (proxy 레이어 소유)
- Depends on: 2.W.1
- Files: `proxy/proxy.go`
- Acceptance: LevelChunk/SubChunk/UpdateBlock 패킷 수신 시 플레이어의 Tracker에 전달.

## Phase 2 / AI-S — Simulation Engine (β)

### Task 2.S.0 — 물리 상수 검증
- Status: **pending**
- Owner: AI-S
- Depends on: 1.10
- Files: `docs/physics-constants.md`
- Acceptance:
  - 설계 §5.2에 정의된 상수 (gravity 0.08, airDrag 0.98, jumpVel 0.42, stepHeight 0.6, friction, item-use speed 등) 각각을 **Oomph 실소스** (`oomph-ac/oomph` master)와 **Dragonfly 플레이어 엔티티 소스**에서 대조.
  - 불일치 시 표에 정리하고 사용자 승인 요청.
  - `basicengine92-tech:refactor/movement-sim` 브랜치도 참조.
- **2.S.2 이후 모든 sim Task의 전제.**

### Task 2.S.1 — SimState · SimInput 정의
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.0
- Files: `anticheat/sim/state.go`
- Acceptance: design.md §5.2.1 구조체 정의, 초기화 헬퍼.

### Task 2.S.2 — Engine 골격
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.1
- Files: `anticheat/sim/engine.go`
- Acceptance: `engine` struct + `Step(prev, input, world)` stub + `meta.SimEngine` 만족.

### Task 2.S.3 — 수평 입력 적용 (WASD)
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.2
- Files: `anticheat/sim/walk.go`
- Acceptance:
  - Forward/Strafe → XZ 가속도 적용.
  - Sprint/Sneak/Swim/UsingItem 승수 적용.
  - 단위 테스트 (각 상태별 기대 속도).

### Task 2.S.4 — 점프
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.3
- Files: `anticheat/sim/jump.go`
- Acceptance:
  - `Jumping && OnGround` → yVel = 0.42.
  - JumpBoost 효과 반영.
  - 스프린트 점프 시 수평 부스트.

### Task 2.S.5 — 중력 · 공기 저항
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.2
- Files: `anticheat/sim/gravity.go`
- Acceptance:
  - `yVel = (yVel - 0.08) * 0.98` 적용.
  - SlowFall / Levitation 분기.
  - 단위 테스트 (10 tick 낙하 궤적).

### Task 2.S.6 — 블록 충돌 (sweep)
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.5, 2.W.5
- Files: `anticheat/sim/collision.go`
- Acceptance:
  - 수평/수직 분리 sweep, AABB vs 주변 BBox.
  - StepHeight 0.6 자동 step.
  - 테스트: 블록 통과·벽 막힘·계단 오르기.

### Task 2.S.7 — 표면별 마찰
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.6
- Files: `anticheat/sim/friction.go`
- Acceptance:
  - 얼음/점액/소울샌드/꿀 각각의 frictionFactor.
  - 발밑 블록 확인 후 승수 적용.

### Task 2.S.8 — 유체
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.5
- Files: `anticheat/sim/fluid.go`
- Acceptance:
  - 물/용암 내 부력·저항·감속.
  - 수영 모드 입력 처리.

### Task 2.S.9 — 기어오르기 블록
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.5
- Files: `anticheat/sim/climbable.go`
- Acceptance:
  - 사다리/덩굴/스캐폴딩 기어오름 속도.
  - Sneak 시 클라이밍 정지 (비-스캐폴딩).

### Task 2.S.10 — 포션 효과
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.3, 2.S.5
- Files: `anticheat/sim/effects.go`
- Acceptance:
  - Speed I~V, JumpBoost I~V, SlowFall, Levitation 전부.
  - 효과 amplifier에 선형 비례.

### Task 2.S.11 — β 통합 테스트
- Status: **pending**
- Owner: AI-S
- Depends on: 2.S.3~2.S.10
- Files: `anticheat/sim/engine_test.go`
- Acceptance:
  - 실제 Bedrock 세션 녹화 (500 tick) 재생 시 diff p99 < 0.02 b/tick.
  - CI 재현 가능.

## Phase 2 / AI-E — Entity Rewind

### Task 2.E.1 — 링버퍼 구현
- Status: **done**
- Owner: AI-E
- Depends on: 1.10
- Files: `anticheat/entity/ringbuffer.go`
- Acceptance:
  - generic ring buffer (or entity-snapshot-specific).
  - 단위 테스트 (push, overflow, query).

### Task 2.E.2 — Rewind 시스템
- Status: **done**
- Owner: AI-E
- Depends on: 2.E.1
- Files: `anticheat/entity/rewind.go`
- Acceptance:
  - `Record / At / Purge` 구현.
  - `At(rid, tick)` 이 정확한 snapshot 또는 직전 snapshot 반환.
  - concurrent access 안전.

### Task 2.E.3 — data.Player 통합
- Status: **pending**
- Owner: AI-O (data.Player 소유)
- Depends on: 2.E.2
- Files: `anticheat/data/player.go`, `anticheat/anticheat.go`
- Acceptance:
  - 기존 `entityPos` 맵 제거.
  - Player가 공유 Rewind 참조 갖기.
  - proxy.serverToClient 가 `Record` 호출.

## Phase 2 / AI-A — Ack System

### Task 2.A.1 — Marker 구조
- Status: **done**
- Owner: AI-A
- Depends on: 1.10
- Files: `anticheat/ack/marker.go`
- Acceptance:
  - `NetworkStackLatency` 패킷 생성 헬퍼.
  - timestamp 할당 정책 (monotonic int64).

### Task 2.A.2 — System 구현
- Status: **done**
- Owner: AI-A
- Depends on: 2.A.1
- Files: `anticheat/ack/system.go`
- Acceptance:
  - `Dispatch(cb) → marker` : 큐에 추가 + 마커 반환.
  - `OnResponse(ts, tick)` : 해당 콜백 실행.
  - 타임아웃 (기본 100 tick) 처리.
  - 동시성 안전.

### Task 2.A.3 — proxy 배선
- Status: **pending**
- Owner: AI-O
- Depends on: 2.A.2
- Files: `proxy/proxy.go`
- Acceptance:
  - serverToClient에서 knockback 등 이벤트 시 `Dispatch` 호출, 마커 패킷을 클라로 보내기.
  - clientToServer에서 `NetworkStackLatency` 응답 수신 시 `OnResponse` 호출.

## Phase 2 / AI-M — Mitigate

### Task 2.M.1 — Policy enum + Dispatcher 인터페이스 구현
- Status: **done**
- Owner: AI-M
- Depends on: 1.10
- Files: `anticheat/mitigate/policy.go`
- Acceptance: `dispatcher` struct + `Apply()` 메서드 stub 형태로 컴파일.

### Task 2.M.2 — Kick 경로
- Status: **done**
- Owner: AI-M
- Depends on: 2.M.1
- Files: `anticheat/mitigate/policy.go`
- Acceptance: VL 초과 시 `kick=true` 반환, 메시지 포맷 통일.

### Task 2.M.3 — ClientRubberband
- Status: **pending**
- Owner: AI-M
- Depends on: 2.M.1, 2.A.2
- Files: `anticheat/mitigate/client_rubberband.go`
- Acceptance:
  - 클라에 `MovePlayer{pos=lastValid, teleport=true}` 발송.
  - ack.Dispatch로 movement 검사 일시 정지 콜백 등록.

### Task 2.M.4 — ServerFilter
- Status: **pending**
- Owner: AI-M
- Depends on: 2.M.1
- Files: `anticheat/mitigate/server_filter.go`
- Acceptance:
  - 원본 패킷의 Position을 마지막 유효 위치로 덮어쓴 copy 반환.
  - 클라에도 correction MovePlayer 발송.

---

# Phase 3 — Check Migration + 신규 (병렬 · AI-C)

**Phase 2 완료 후 착수**. AI-C를 5개 서브로 나눠 병렬 가능.

## Phase 3 / AI-C-1 — Movement 체크

### Task 3.C1.1 — Speed/A 재작성 (sim-based)
- Status: **pending** · Depends on: 2.S.11, 2.M.1
### Task 3.C1.2 — Speed/B 재작성
- Status: **pending** · Depends on: 3.C1.1
### Task 3.C1.3 — Fly/A 재작성
- Status: **pending** · Depends on: 2.S.11
### Task 3.C1.4 — Fly/B 재작성
- Status: **pending** · Depends on: 3.C1.3
### Task 3.C1.5 — Fly/C (유체) 신규
- Status: **pending** · Depends on: 3.C1.3, 2.S.8
### Task 3.C1.6 — Phase/A 재작성 (BBox 기반)
- Status: **pending** · Depends on: 2.S.6
### Task 3.C1.7 — NoFall/A 재작성
- Status: **pending** · Depends on: 2.S.11
### Task 3.C1.8 — NoFall/B 재작성
- Status: **pending** · Depends on: 2.S.11
### Task 3.C1.9 — Step/A 신규
- Status: **pending** · Depends on: 2.S.6
### Task 3.C1.10 — HighJump/A 신규
- Status: **pending** · Depends on: 2.S.4
### Task 3.C1.11 — Jesus/A 신규
- Status: **pending** · Depends on: 2.S.8
### Task 3.C1.12 — Spider/A 신규
- Status: **pending** · Depends on: 2.S.6, 2.S.9
### Task 3.C1.13 — InvalidMove/A 신규
- Status: **pending** · Depends on: 1.10
### Task 3.C1.14 — NoSlow/A 개조
- Status: **pending** · Depends on: 2.S.11
### Task 3.C1.15 — Velocity/A 개조
- Status: **pending** · Depends on: 2.A.2, 2.S.11

공통 acceptance: 해당 체크의 `docs/check-specs/<key>.md` 먼저 작성, 단위 테스트 3종(합법/치트/경계) 포함, `Policy()` 반환 확정.

## Phase 3 / AI-C-2 — Combat 체크

### Task 3.C2.1 — Reach/A 개조 (rewind)
- Status: **pending** · Depends on: 2.E.2
### Task 3.C2.2 — Reach/B 신규 (raycast)
- Status: **pending** · Depends on: 2.W.5, 2.E.2
### Task 3.C2.3 — KillAura/A 유지 (Policy 추가)
- Status: **pending**
### Task 3.C2.4 — KillAura/B 개조 (FOV rewind)
- Status: **pending** · Depends on: 2.E.2
### Task 3.C2.5 — KillAura/C 유지
- Status: **pending**
### Task 3.C2.6 — KillAura/D 신규 (yaw-snap)
- Status: **pending**
### Task 3.C2.7 — AutoClicker/A 유지
- Status: **pending**
### Task 3.C2.8 — AutoClicker/B 유지
- Status: **pending**
### Task 3.C2.9 — AutoClicker/C 신규 (double-click)
- Status: **pending**
### Task 3.C2.10 — Aim/A 유지
- Status: **pending**
### Task 3.C2.11 — Aim/B 유지
- Status: **pending**

## Phase 3 / AI-C-3 — Packet 체크

### Task 3.C3.1 ~ 3.C3.5 — BadPacket/A~E 유지 (Policy 추가)
- Status: **pending**
### Task 3.C3.6 — BadPacket/F 신규 (누락 플래그)
- Status: **pending**
### Task 3.C3.7 — BadPacket/G 신규 (패킷 순서)
- Status: **pending**
### Task 3.C3.8 — Timer/A 유지
- Status: **pending**

## Phase 3 / AI-C-4 — World 체크 (신규 카테고리)

### Task 3.C4.0 — `feat/scaffold-dtc` 딥다이브
- Status: **pending**
- Owner: AI-C-4
- Depends on: 1.10
- Files: `docs/check-specs/world-scaffold-a.md`
- Acceptance:
  - `basicengine92-tech/oomph` 의 `feat/scaffold-dtc` 브랜치 36개 커밋 전수 조사.
  - 알고리즘 요약, 채택할 idea와 거르는 idea 구분, 오탐 방지 장치.
  - 이 문서 없이는 3.C4.1 시작 금지.

### Task 3.C4.1 — Scaffold/A 개조
- Status: **pending** · Depends on: 2.W.5, 3.C4.0
### Task 3.C4.2 — Nuker/A 신규 (다중 블록)
- Status: **pending** · Depends on: 2.W.5
### Task 3.C4.3 — Nuker/B 신규 (범위)
- Status: **pending** · Depends on: 2.W.5
### Task 3.C4.4 — FastBreak/A 신규
- Status: **pending** · Depends on: 2.W.5
### Task 3.C4.5 — FastPlace/A 신규
- Status: **pending**
### Task 3.C4.6 — Tower/A 신규
- Status: **pending** · Depends on: 2.W.5
### Task 3.C4.7 — InvalidBreak/A 신규
- Status: **pending** · Depends on: 2.W.5

## Phase 3 / AI-C-5 — Client 체크 (신규 카테고리)

### Task 3.C5.1 — EditionFaker/A 신규
- Status: **pending**
### Task 3.C5.2 — ClientSpoof/A 신규
- Status: **pending**
### Task 3.C5.3 — Protocol/A 신규
- Status: **pending**

---

# Phase 4 — γ Physics 확장 (병렬 · AI-S)

**Phase 2 완료 후 착수 가능** (Phase 3 병렬).

### Task 4.1 — 엘리트라 글라이딩
- Status: **pending** · Depends on: 2.S.11
- Files: `anticheat/sim/elytra.go`
### Task 4.2 — 트라이던트 릿프타이드
- Status: **pending** · Depends on: 2.S.11
### Task 4.3 — 윈드차지
- Status: **pending** · Depends on: 2.S.11
### Task 4.4 — 가루눈 sink/climb
- Status: **pending** · Depends on: 2.S.5, 2.S.9
### Task 4.5 — 꿀 블록 슬라이드
- Status: **pending** · Depends on: 2.S.7
### Task 4.6 — 거미줄 full slow
- Status: **pending** · Depends on: 2.S.3, 2.S.5
### Task 4.7 — 스캐폴딩 블록 sneak/jump
- Status: **pending** · Depends on: 2.S.9
### Task 4.8 — 슬라임 바운스 체인
- Status: **pending** · Depends on: 2.S.6, 2.S.7

---

# Phase 5a — Integration (순차 · AI-O)

### Task 5a.1 — Manager 배선
- Status: **pending**
- Owner: AI-O
- Depends on: Phase 2 · Phase 3 Movement/Combat/Packet 완료
- Files: `anticheat/anticheat.go`
- Acceptance: world/sim/entity/ack/mitigate 전부 Manager에서 생성·주입·호출.

### Task 5a.2 — proxy 통합
- Status: **pending**
- Depends on: 5a.1, 2.W.7, 2.A.3
- Files: `proxy/proxy.go`
- Acceptance: 모든 패킷 훅 배선, correction 발송 경로 동작.

### Task 5a.3 — 통합 테스트
- Status: **pending**
- Depends on: 5a.2
- Files: `anticheat/integration_test.go`, `testdata/sessions/*`
- Acceptance:
  - 합법 플레이어 녹화 5세션 → 0 violation.
  - 치트 시뮬 녹화 5세션 (Speed/Fly/Reach/Scaffold/Nuker) → 각 해당 체크 flagged.

### Task 5a.4 — 성능 프로파일링 & 벤치
- Status: **pending**
- Depends on: 5a.3
- Files: `bench/loadtest.go`
- Acceptance:
  - design.md §14.3 벤치 3종 (100 CCU / 치트 분류 / 1시간 leak) 모두 통과.
  - p99 tick < 20ms, 메모리 p95 < 2GB RSS.

# Phase 5b — Release (순차 · AI-O)

### Task 5b.1 — CI / GitHub Actions
- Status: **pending**
- Depends on: 5a.4
- Files: `.github/workflows/ci.yml`
- Acceptance:
  - push / PR에서 `go build ./... && go vet ./... && go test -race ./...` 실행.
  - Linux + Windows 매트릭스.
  - 벤치 스모크 1종 nightly.

### Task 5b.2 — 메트릭 & 구조화 로그
- Status: **pending**
- Depends on: 5a.1
- Files: `anticheat/mitigate/metrics.go`, `docs/ops-runbook.md`
- Acceptance:
  - design.md §13 메트릭 6종 전부 노출.
  - `/metrics` HTTP 엔드포인트 (옵션 플래그).
  - 운영 매뉴얼 작성 (flag 포맷, 메트릭 의미, 대시보드 JSON 샘플).

### Task 5b.3 — β 릴리스
- Status: **pending**
- Depends on: 5a.4, 5b.1
- Acceptance: γ 기능 disabled, 태그 `v0.10.0-beta` 생성, CHANGELOG 작성.

### Task 5b.4 — γ 릴리스
- Status: **pending**
- Depends on: 5b.3, Phase 4 전체
- Acceptance: γ 기능 enabled, 태그 `v0.10.0` 생성.

---

# 완료된 Task (아카이브)

- 2026-04-19 / Task 1.1 (AI-O) — 설계 문서 작성.
- 2026-04-19 / Task 1.2 (AI-O) — 다중 AI 협업 문서 작성.

---

**끝.** 새 Task는 해당 Phase 섹션에 추가. 완료된 Task는 위 아카이브로 이동.
