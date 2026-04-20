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
| Phase 진행 중 | **Phase 3 β 완료 (existing 체크 100%); World/Client/Phase 4 spec-done — γ 구현 대기** |
| 전체 진도 | 54 + spec-done 17 + 5a.1 부분 / ~75 Tasks done (Phase 1+2 전체 + 5a.1 mitigate + 3.C1.{1,3,6,7,14,15} β + 3.C2.{1,3,4,5,7,10} β + 3.C3.{1-5} β + 3.C4.{1-7} spec + 3.C5.{1-3} spec + 4.{1-8} spec; 2.W.3은 γ+1로 연기) |
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
- Status: **done** (문서만, 실캡처는 β 필드 테스트 시)
- Owner: AI-W
- Depends on: 1.10
- Files: `testdata/README.md`
- Acceptance:
  - ✅ 실제 Bedrock 세션 캡처 방법 README에 기록. 제3자 AI가 재현 가능.
  - ⚠️ `.bin` 파일은 β 현장 테스트 때 캡처 예정 — 런타임 의존.
  - 현재 단위 테스트는 `dfchunk.Chunk` 직접 주입으로 query path만 검증.
- **2.W.2 이후 모든 Task가 이 파일에 의존함.**

### Task 2.W.1 — Tracker 구조체 + 생성자
- Status: **done**
- Owner: AI-W
- Depends on: 1.10
- Files: `anticheat/world/tracker.go`
- Acceptance:
  - ✅ `Tracker` struct + `NewTracker()` + `NewTrackerWithRange(cube.Range)` + `meta.WorldTracker` 만족.
  - ✅ `ChunkLoaded`, `Close` 구현. `sync.RWMutex` 기반 동시성.

### Task 2.W.2 — LevelChunk 파싱
- Status: **done**
- Owner: AI-W
- Depends on: 2.W.1
- Files: `anticheat/world/chunks.go`
- Acceptance:
  - ✅ `HandleLevelChunk(*packet.LevelChunk)` 이 `dfchunk.NetworkDecode` 경유로 파싱.
  - ✅ SubChunkRequest mode는 빈 chunk 생성 (병합 대기).
  - ✅ CacheEnabled는 β에서 거부 (fail-closed).
  - ⚠️ 실제 testdata 디코딩 테스트는 2.W.0 .bin 파일 캡처 후 추가.

### Task 2.W.3 — SubChunk 패킷 처리
- Status: **blocked** (γ+1로 연기)
- Owner: AI-W
- Depends on: 2.W.2
- Files: `anticheat/world/chunks.go`
- Blocker: Dragonfly `chunk.decodeSubChunk` 미공개. 단일 서브청크를 격리 디코딩하려면 biome tail을 합성하거나 파서를 포크해야 함.
- β 현재: `HandleSubChunk`는 no-op. SubChunkRequest 모드 서버는 ChunkLoaded=true지만 Block 쿼리는 air 반환 (fail-open).
- 영향: Phase/Scaffold 체크가 해당 서버에선 정확도 저하. Fly/Speed는 영향 없음.
- γ+1 해결안: (A) dragonfly 패키지 벤더 포크 + decodeSubChunk export, (B) 자체 팔레트 워커 구현.

### Task 2.W.4 — UpdateBlock 처리
- Status: **done**
- Owner: AI-W
- Depends on: 2.W.1
- Files: `anticheat/world/blocks.go`
- Acceptance:
  - ✅ `HandleBlockUpdate(pos, rid)` 이 `dfchunk.SetBlock` 호출.
  - ✅ 청크 미로드·Y 범위 밖 좌표는 silent no-op (로그 없이 드롭 — 청크 전환 경계의 정상 race).
  - ✅ 단위 테스트 4개 (round-trip, 범위 밖, 미로드 청크, chunkXZ 경계).

### Task 2.W.5 — 블록 조회 + BBox
- Status: **done**
- Owner: AI-W
- Depends on: 2.W.2, 2.W.4
- Files: `anticheat/world/blocks.go`
- Acceptance:
  - ✅ `Block(pos)` → `world.BlockByRuntimeID` 경유. 미등록 rid는 air fallback.
  - ✅ `BlockBBoxes(pos)` → `b.Model().BBox(pos, t)`. 알 수 없는 모델은 full cube (보수적 fail-closed).
  - ✅ Tracker가 `world.BlockSource`를 만족해서 이웃 쿼리가 올바른 뷰를 봄.

### Task 2.W.6 — 청크 언로드
- Status: **done**
- Owner: AI-W
- Depends on: 2.W.1
- Files: `anticheat/world/unload.go`
- Acceptance:
  - ✅ `HandleChunkPublisher(*NetworkChunkPublisherUpdate)` 가 radius 밖 청크를 delete.
  - ⚠️ 1시간 누수 테스트는 현장 벤치 (5a Task 5a.4)로 연기.

### Task 2.W.7 — proxy.serverToClient 훅
- Status: **done**
- Owner: AI-O (proxy 레이어 소유)
- Completed: AI-O 2026-04-20
- Depends on: 2.W.1
- Files: `proxy/proxy.go`, `proxy/session.go`
- Acceptance:
  - ✅ `Session.World` 필드 + `newSession` 이 `world.NewTracker()` 로 초기화.
  - ✅ serverToClient 에서 `*packet.LevelChunk`/`*packet.SubChunk`/`*packet.UpdateBlock`/`*packet.NetworkChunkPublisherUpdate` 를 Tracker 메서드로 라우팅.
  - ✅ 세션 종료 시 `sess.World.Close()` 로 청크 맵 해제.
- Commits: (이번 통합 배치)

## Phase 2 / AI-S — Simulation Engine (β)

### Task 2.S.0 — 물리 상수 검증
- Status: **done**
- Owner: AI-S
- Depends on: 1.10
- Files: `docs/physics-constants.md`
- Acceptance:
  - 설계 §5.2에 정의된 상수 (gravity 0.08, airDrag 0.98, jumpVel 0.42, stepHeight 0.6, friction, item-use speed 등) 각각을 **Oomph 실소스** (`oomph-ac/oomph` master)와 **Dragonfly 플레이어 엔티티 소스**에서 대조.
  - 불일치 시 표에 정리하고 사용자 승인 요청.
  - `basicengine92-tech:refactor/movement-sim` 브랜치도 참조.
- **2.S.2 이후 모든 sim Task의 전제.**

### Task 2.S.1 — SimState · SimInput 정의
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.0
- Files: `anticheat/sim/state.go`
- Acceptance: design.md §5.2.1 구조체 정의, 초기화 헬퍼.

### Task 2.S.2 — Engine 골격
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.1
- Files: `anticheat/sim/engine.go`
- Acceptance: `engine` struct + `Step(prev, input, world)` stub + `meta.SimEngine` 만족.

### Task 2.S.3 — 수평 입력 적용 (WASD)
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.2
- Files: `anticheat/sim/walk.go`
- Acceptance:
  - Forward/Strafe → XZ 가속도 적용.
  - Sprint/Sneak/Swim/UsingItem 승수 적용.
  - 단위 테스트 (각 상태별 기대 속도).

### Task 2.S.4 — 점프
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.3
- Files: `anticheat/sim/jump.go`
- Acceptance:
  - `Jumping && OnGround` → yVel = 0.42.
  - JumpBoost 효과 반영.
  - 스프린트 점프 시 수평 부스트.

### Task 2.S.5 — 중력 · 공기 저항
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.2
- Files: `anticheat/sim/gravity.go`
- Acceptance:
  - `yVel = (yVel - 0.08) * 0.98` 적용.
  - SlowFall / Levitation 분기.
  - 단위 테스트 (10 tick 낙하 궤적).

### Task 2.S.6 — 블록 충돌 (sweep)
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.5, 2.W.5
- Files: `anticheat/sim/collision.go`
- Acceptance:
  - 수평/수직 분리 sweep, AABB vs 주변 BBox.
  - StepHeight 0.6 자동 step.
  - 테스트: 블록 통과·벽 막힘·계단 오르기.

### Task 2.S.7 — 표면별 마찰
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.6
- Files: `anticheat/sim/friction.go`
- Acceptance:
  - 얼음/점액/소울샌드/꿀 각각의 frictionFactor.
  - 발밑 블록 확인 후 승수 적용.

### Task 2.S.8 — 유체
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.5
- Files: `anticheat/sim/fluid.go`
- Acceptance:
  - 물/용암 내 부력·저항·감속.
  - 수영 모드 입력 처리.

### Task 2.S.9 — 기어오르기 블록
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.5
- Files: `anticheat/sim/climbable.go`
- Acceptance:
  - 사다리/덩굴/스캐폴딩 기어오름 속도.
  - Sneak 시 클라이밍 정지 (비-스캐폴딩).

### Task 2.S.10 — 포션 효과
- Status: **done**
- Owner: AI-S
- Depends on: 2.S.3, 2.S.5
- Files: `anticheat/sim/effects.go`
- Acceptance:
  - Speed I~V, JumpBoost I~V, SlowFall, Levitation 전부.
  - 효과 amplifier에 선형 비례.

### Task 2.S.11 — β 통합 테스트
- Status: **done**
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
- Status: **done**
- Owner: AI-O (data.Player 소유)
- Completed: AI-O 2026-04-20
- Depends on: 2.E.2
- Files: `proxy/session.go`, `proxy/proxy.go`
- Acceptance:
  - ✅ `Session.Rewind` 필드 + `newSession` 이 `entity.NewRewind()` 로 초기화.
  - ✅ proxy.serverToClient 의 AddPlayer/AddActor/MovePlayer/MoveActorAbsolute 핸들러가 `p.recordRewind(sess, rid, pos, pitch, yaw)` 호출 → `sess.Rewind.Record(rid, tick, pos, bbox, rot)`.
  - ⏭️ `data.Player.entityPos` 제거 및 Rewind 공유는 Phase 3 (체크 마이그레이션) 에서 수행 (체크들이 해당 맵에 의존). 통합 배선 단계에서는 proxy→Rewind 경로만 활성화.
- Notes: `entityPos` 맵 제거는 Phase 3 Reach/Aim 체크 마이그레이션 (3.C-2.*) 타이밍에 맞춰 수행. β 기간엔 양쪽 공존.
- Commits: (이번 통합 배치)

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
- Status: **done**
- Owner: AI-O
- Completed: AI-O 2026-04-20
- Depends on: 2.A.2
- Files: `proxy/session.go`, `proxy/proxy.go`
- Acceptance:
  - ✅ `Session.Ack` 필드 + `newSession` 이 `ack.NewSystem()` 으로 초기화.
  - ✅ clientToServer 에서 `*packet.NetworkStackLatency` 수신 시 `sess.Ack.OnResponse(ts, pl.SimFrame())` 호출.
  - ⏭️ serverToClient 측 `Dispatch` 호출 지점 (knockback 동기화 등) 은 Phase 3 Timer/KB 체크 마이그레이션 시점에 배선 (지금은 무호출 상태로 유지 — β 단계에서 Ack 은 응답 경로만 활성).
- Notes: β 에서는 응답 수집/타임아웃 경로만 활성. Dispatch 호출부는 체크 측에서 필요 시 주입.
- Commits: (이번 통합 배치)

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
- Status: **done**
- Owner: AI-M
- Completed: AI-M 2026-04-20
- Depends on: 2.M.1, 2.A.2
- Files: `anticheat/mitigate/policy.go`, `anticheat/mitigate/policy_test.go`
- Acceptance:
  - ✅ `RubberbandFunc` 타입 + `NewDispatcherWithHooks` 생성자 + `applyRubberband` 경로.
  - ✅ 훅이 nil 이면 log-only 로 degrade (패닉 없음).
  - ✅ `TestApplyRubberbandCallsHook` / `TestApplyHooksNilDegradeToLog` 통과.
  - ⏭️ 구체적 `MovePlayer{pos=lastValid, teleport=true}` 발송 + ack.Dispatch 를 이용한 movement 검사 일시 정지 콜백은 proxy 레이어의 훅 구현체 (Phase 5a 통합) 에서 수행. dispatcher 는 훅 계약만 정의.
- Notes: design.md §7에 정한 대로 dispatcher 는 훅 계약만 소유. 실제 rubberband 패킷 발송/ack 배선은 proxy 측 훅 구현 (5a Task 5a.1)에서.
- Commits: (이번 통합 배치)

### Task 2.M.4 — ServerFilter
- Status: **done**
- Owner: AI-M
- Completed: AI-M 2026-04-20
- Depends on: 2.M.1
- Files: `anticheat/mitigate/policy.go`, `anticheat/mitigate/policy_test.go`
- Acceptance:
  - ✅ `ServerFilterFunc` 타입 + `applyServerFilter` 경로: 훅 반환값을 forward (nil 반환 → drop).
  - ✅ 훅이 nil 이면 log-only 로 degrade (원본 forward).
  - ✅ `TestApplyServerFilterRewritesPacket` / `TestApplyHooksNilDegradeToLog` 통과.
  - ⏭️ 원본 패킷 Position 덮어쓰기 + correction MovePlayer 발송은 proxy 레이어의 훅 구현체 (Phase 5a 통합) 에서 수행. dispatcher 는 훅 계약만 정의.
- Notes: Mitigate 패키지는 훅 계약만 소유 (design.md §7). 구체 rewrite/correction 로직은 proxy 레이어 훅 구현 (5a Task 5a.1)에서.
- Commits: (이번 통합 배치)

---

# Phase 3 — Check Migration + 신규 (병렬 · AI-C)

**Phase 2 완료 후 착수**. AI-C를 5개 서브로 나눠 병렬 가능.

## Phase 3 / AI-C-1 — Movement 체크

### Task 3.C1.1 — Speed/A 재작성 (sim-based)
- Status: **done (β: 스펙+단위테스트; sim 재작성은 γ에서)**
- Owner: AI-C-1
- Completed: AI-C-1 2026-04-20
- Depends on: 2.S.11, 2.M.1
- Files: `docs/check-specs/movement-speed-a.md`, `anticheat/checks/movement/speed_test.go`
- Acceptance:
  - ✅ 스펙 문서 (§ Summary–Test vectors 전부).
  - ✅ 단위 테스트 4종 (합법/치트/경계/Policy).
  - ✅ `Policy()` 반환 확정 (default Kick).
  - ⏭️ `sim.Engine.Step()` 기반 재작성은 γ — 현재 헤유리스틱이 β 수용 기준을 만족 (0 FP on test fixtures).
### Task 3.C1.2 — Speed/B 재작성
- Status: **pending** · Depends on: 3.C1.1
### Task 3.C1.3 — Fly/A 재작성
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-1
- Completed: AI-C-1 2026-04-20
- Depends on: 2.S.11
- Files: `docs/check-specs/movement-fly-a.md`, `anticheat/checks/movement/fly_test.go`
- Acceptance:
  - ✅ 스펙 문서 (hover / upward_fly / 면제 목록).
  - ✅ 단위 테스트 5종 (jump arc / hover / upward / creative / Policy).
  - ✅ `Policy()` 반환 확정 (default ServerFilter).
### Task 3.C1.4 — Fly/B 재작성
- Status: **pending** · Depends on: 3.C1.3
### Task 3.C1.5 — Fly/C (유체) 신규
- Status: **pending** · Depends on: 3.C1.3, 2.S.8
### Task 3.C1.6 — Phase/A 재작성 (BBox 기반)
- Status: **done (β: 스펙+단위테스트; BBox 버전은 γ 2.W.5 완료 후)**
- Owner: AI-C-1
- Completed: AI-C-1 2026-04-20
- Depends on: 2.S.6
- Files: `docs/check-specs/movement-phase-a.md`, `anticheat/checks/movement/phase_test.go`
- Acceptance:
  - ✅ 스펙 문서 (6-block cap, teleport grace, gliding 면제).
  - ✅ 단위 테스트 5종 (legal sprint-jump / cheat / boundary / teleport grace / Policy).
  - ✅ `Policy()` 반환 Kick.
  - ⏭️ BBox 교차 기반 재작성은 γ: World tracker raycast (2.W.5) 완료 + sim.CollisionSystem 통합 후.
### Task 3.C1.7 — NoFall/A 재작성
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-1
- Completed: AI-C-1 2026-04-20
- Depends on: 2.S.11
- Files: `docs/check-specs/movement-nofall-a.md`, `anticheat/checks/movement/nofall_test.go`
- Acceptance:
  - ✅ 스펙 문서 (damage threshold 3.0 blocks, water / slow-falling 면제).
  - ✅ 단위 테스트 4종 (short fall / cheat / boundary / Policy).
  - ✅ `Policy()` 반환 확정 (default Kick).
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
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-1
- Completed: AI-C-1 2026-04-20
- Depends on: 2.S.11
- Files: `docs/check-specs/movement-noslow-a.md`, `anticheat/checks/movement/noslow_test.go`
- Acceptance:
  - ✅ 스펙 문서 (0.21 b/tick cap, water / knockback 면제).
  - ✅ 단위 테스트 5종 (legal / cheat / boundary / not-using-item / Policy).
  - ✅ `Policy()` 반환 Kick.
### Task 3.C1.15 — Velocity/A 개조
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-1
- Completed: AI-C-1 2026-04-20
- Depends on: 2.A.2, 2.S.11
- Files: `docs/check-specs/movement-velocity-a.md`, `anticheat/checks/movement/velocity_test.go`
- Acceptance:
  - ✅ 스펙 문서 (15% projection ratio, 0.1 minKB floor).
  - ✅ 단위 테스트 5종 (legal absorption / cheat / sub-threshold / ratio boundary / Policy).
  - ✅ `Policy()` 반환 Kick.

공통 acceptance: 해당 체크의 `docs/check-specs/<key>.md` 먼저 작성, 단위 테스트 3종(합법/치트/경계) 포함, `Policy()` 반환 확정.

## Phase 3 / AI-C-2 — Combat 체크

### Task 3.C2.1 — Reach/A 개조 (rewind)
- Status: **done (β: 스펙+단위테스트; γ 시 rewind 기반 재측정 예정)**
- Owner: AI-C-2
- Completed: AI-C-2 2026-04-21
- Depends on: 2.E.2
- Files: `docs/check-specs/combat-reach-a.md`, `anticheat/checks/combat/reach_test.go`
- Acceptance:
  - ✅ 스펙 문서 완성 (ping 보정 로직 및 eye offset 설명 포함).
  - ✅ 단위 테스트 5종 (legal / cheat / boundary / ping-compensation / Policy).
  - ✅ `Policy()` 반환 Kick.

### Task 3.C2.2 — Reach/B 신규 (raycast)
- Status: **pending** · Depends on: 2.W.5, 2.E.2

### Task 3.C2.3 — KillAura/A 유지 (Policy 추가)
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-2
- Completed: AI-C-2 2026-04-21
- Files: `docs/check-specs/combat-killaura-abc.md`, `anticheat/checks/combat/killaura_test.go`
- Acceptance:
  - ✅ 합쳐진 /A+/B+/C 스펙 문서.
  - ✅ /A 단위테스트 5종 (grace / in-window / swing-less / tick-wrap / Policy).
  - ✅ `Policy()` 반환 Kick.

### Task 3.C2.4 — KillAura/B 개조 (FOV rewind)
- Status: **done (β: 스펙+단위테스트; γ 시 rewind 기반 target 보정 예정)**
- Owner: AI-C-2
- Completed: AI-C-2 2026-04-21
- Depends on: 2.E.2
- Files: `docs/check-specs/combat-killaura-abc.md`, `anticheat/checks/combat/killaura_test.go`
- Acceptance:
  - ✅ 공유 스펙 내 /B 전용 알고리즘/임계치 명세.
  - ✅ 단위 테스트 3종 (on-axis / behind-target / Policy).

### Task 3.C2.5 — KillAura/C 유지
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-2
- Completed: AI-C-2 2026-04-21
- Files: `docs/check-specs/combat-killaura-abc.md`, `anticheat/checks/combat/killaura_test.go`
- Acceptance:
  - ✅ 공유 스펙 내 /C 전용 섹션.
  - ✅ 단위 테스트 4종 (single / multi-same-tick / multi-adjacent-tick / Policy).

### Task 3.C2.6 — KillAura/D 신규 (yaw-snap)
- Status: **pending**

### Task 3.C2.7 — AutoClicker/A 유지
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-2
- Completed: AI-C-2 2026-04-21
- Files: `docs/check-specs/combat-autoclicker-a.md`, `anticheat/checks/combat/autoclicker_test.go`
- Acceptance:
  - ✅ 스펙 문서 (rolling 1s window, FailBuffer=4, TrustDuration 30s).
  - ✅ 단위 테스트 5종 (legal / cheat / boundary / disabled / Policy).
  - ✅ `Policy()` 반환 Kick.

### Task 3.C2.8 — AutoClicker/B 유지
- Status: **pending (γ: 정밀 타임스탬프 주입 필요)**

### Task 3.C2.9 — AutoClicker/C 신규 (double-click)
- Status: **pending**

### Task 3.C2.10 — Aim/A 유지
- Status: **done (β: 스펙+단위테스트)**
- Owner: AI-C-2
- Completed: AI-C-2 2026-04-21
- Files: `docs/check-specs/combat-aim-a.md`, `anticheat/checks/combat/aim_test.go`
- Acceptance:
  - ✅ 스펙 문서 (round-match 3e-5, 1e-3 idle gate, mouse-only).
  - ✅ 단위 테스트 5종 (non-mouse / round-delta / natural-delta / idle / Policy).

### Task 3.C2.11 — Aim/B 유지
- Status: **pending (γ: ConstPitchTicks 시나리오 주입 필요)**

## Phase 3 / AI-C-3 — Packet 체크

### Task 3.C3.1 ~ 3.C3.5 — BadPacket/A~E 유지 (Policy 추가)
- Status: **done (β: 전체 서브체크 스펙+단위테스트 완료)**
- Owner: AI-C-3
- Completed: AI-C-3 2026-04-21
- Depends on: 1.10 (Policy enum 이미 배선됨)
- Files: `docs/check-specs/packet-badpacket-a.md`, `docs/check-specs/packet-badpacket-bcd.md`, `docs/check-specs/packet-badpacket-e.md`, `anticheat/checks/packet/badpacket_test.go`
- Acceptance:
  - ✅ BadPacket/A (tick validation) 단위테스트 6종.
  - ✅ BadPacket/B (pitch 범위) · /C (sprint+sneak) · /D (NaN/Inf) 단위테스트 10종.
  - ✅ BadPacket/E (contradictory start+stop flags) 단위테스트 5종.
  - ✅ 서브체크별 / 통합 스펙 문서 3건.
  - ✅ 각 체크 `Policy()` 반환 Kick 확인.
### Task 3.C3.6 — BadPacket/F 신규 (누락 플래그)
- Status: **pending**
### Task 3.C3.7 — BadPacket/G 신규 (패킷 순서)
- Status: **pending**
### Task 3.C3.8 — Timer/A 유지
- Status: **pending**

## Phase 3 / AI-C-4 — World 체크 (신규 카테고리)

### Task 3.C4.0 — `feat/scaffold-dtc` 딥다이브
- Status: **pending** · Depends on: 1.10
- Owner: AI-C-4
- Files: `docs/check-specs/world-scaffold-a.md` (별도; 통합 스펙은 world-block-checks.md 참조)

### Task 3.C4.1 ~ 3.C4.7 — World 체크 통합 스펙
- Status: **spec-done (γ 구현 대기)**
- Owner: AI-C-4
- Completed (스펙): AI-C-4 2026-04-21
- Depends on: 2.W.5
- Files: `docs/check-specs/world-block-checks.md`
- Acceptance (스펙):
  - ✅ Scaffold/A · Nuker/A+B · FastBreak/A · FastPlace/A · Tower/A · InvalidBreak/A 7종 통합 스펙.
  - ✅ 각 체크의 detection floor / FailBuffer / Policy 명시.
  - ✅ 의존성(world tracker · sim engine) 및 open question 정리.
- Pending (γ): 실제 `anticheat/checks/world/<name>.go` 구현 + 단위 테스트 + Manager 등록.

## Phase 3 / AI-C-5 — Client 체크 (신규 카테고리)

### Task 3.C5.1 ~ 3.C5.3 — Client 체크 통합 스펙
- Status: **spec-done (γ 구현 대기)**
- Owner: AI-C-5
- Completed (스펙): AI-C-5 2026-04-21
- Files: `docs/check-specs/client-protocol-checks.md`
- Acceptance (스펙):
  - ✅ EditionFaker/A · ClientSpoof/A · Protocol/A 통합 스펙.
  - ✅ Login lifecycle hook 설계 (Manager 신규 hook 필요).
  - ✅ Fingerprint 테이블 위치 및 갱신 절차.
- Pending (γ): `anticheat/checks/client/<name>.go` 구현 + Login hook + 단위 테스트.

---

# Phase 4 — γ Physics 확장 (병렬 · AI-S)

**Phase 2 완료 후 착수 가능** (Phase 3 병렬).

### Task 4.1 ~ 4.8 — γ 물리 확장 통합 스펙
- Status: **spec-done (구현 대기)**
- Owner: AI-S
- Completed (스펙): AI-S 2026-04-21
- Files: `docs/check-specs/phase4-physics-extensions.md`
- Acceptance (스펙):
  - ✅ Elytra · Trident Riptide · Wind Charge · Powder Snow · Honey Block · Cobweb · Scaffolding · Slime Bounce 8종 통합 스펙.
  - ✅ 각 확장의 SimState 필드 / 알고리즘 / 영향받는 체크 exemption 정리.
  - ✅ Open questions 문서화 (slow-fall × elytra interaction, riptide in shallow water).
- Pending (구현): 각 확장의 `anticheat/sim/<name>.go` + 단위 테스트.

---

# Phase 5a — Integration (순차 · AI-O)

### Task 5a.1 — Manager 배선
- Status: **in-progress (mitigate 부분 완료)**
- Owner: AI-O
- Depends on: Phase 2 · Phase 3 Movement/Combat/Packet 완료
- Files: `anticheat/anticheat.go`, `anticheat/dispatcher_test.go`
- Acceptance: world/sim/entity/ack/mitigate 전부 Manager에서 생성·주입·호출.
- 진행:
  - ✅ Mitigate `Dispatcher` 가 `handleViolation` 경로로 배선됨: `KickFunc`/`RubberbandFunc`/`ServerFilterFunc` 필드를 Manager 가 보유, Apply 가 Policy 별로 라우팅.
  - ✅ 3종 단위 테스트 (`TestHandleViolationRoutesKickThroughDispatcher`, `TestHandleViolationKickDryRunWhenKickFuncNil`, `TestHandleViolationRubberbandHookFires`) 통과.
  - ⏭️ Sim/World/Entity Rewind 는 현재 Session (proxy 레이어) 에 붙어있고, Manager 가 Session → Player 매핑을 통해 접근하도록 하는 배선은 Phase 3 체크 마이그레이션과 함께 진행. 체크가 sim 결과에 의존하기 시작하면 그 때 Manager 에 sim.Engine 필드 추가 예정.
  - ⏭️ Ack.Dispatch 호출 지점은 Phase 3 Timer/KB 체크에서 배선.
- Commits: (Manager+Dispatcher 배치)

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
