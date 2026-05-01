# Oomph 대비 탐지 권위성 추월 — 구현 계획 (v1.0-rc)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Better-pm-AC β 골격을 dual-tick + server-authoritative movement + multi-raycast combat 모델로 in-place 업그레이드해 PMMP 5.x + MCBE 1.21.x 환경에서 Oomph보다 낮은 false-positive와 더 넓은 커버리지를 달성한다.

**Architecture:** §1–§9 설계는 `docs/plans/2026-04-21-oomph-parity-plus-design.md` (commit `1a2af17`)에 동결. 이 문서는 그 설계를 task 단위로 분해한 실행본. 모든 변화는 feature flag 뒤에서 점진 활성화하며, 각 phase 끝마다 shippable 상태를 유지한다. 외부 의존성 추가 없음 (Gophertunnel/dragonfly/expvar 그대로).

**Tech Stack:** Go 1.24, [Gophertunnel](https://github.com/Sandertv/gophertunnel) (RakNet/MCBE 프로토콜), [dragonfly](https://github.com/df-mc/dragonfly) (블록 레지스트리), `expvar` (메트릭), `mathgl/mgl32` (벡터). 테스트는 `go test`, 벤치는 `go test -bench=.`, 린트는 `golangci-lint v1.62`.

**Skills:** TDD discipline는 [@superpowers:test-driven-development]. 각 task의 명시된 verify 명령은 [@superpowers:verification-before-completion] 의 게이트.

**Phase 매핑:**

| Phase | 기간 | Task 범위 | 게이트 |
|---|---|---|---|
| γ.1 | W1–2 | T1.1–T1.10 | β 회귀 0 (shadow 모드) |
| γ.2 | W2–3 | T2.1–T2.10 | scenario 20개 중 legit 10개 FP=0 |
| γ.3 | W3–5 | T3.1–T3.12 | reconcile snap rate 예측 범위 |
| γ.4 | W5–6 | T4.1–T4.08 | 치트 재생 범주별 flag ≥ 1 |
| γ.5 | W6–7 | T5.1–T5.10 | v1.0-rc 성공 지표 표 전부 green |
| γ.6 | W7–8 | T6.1–T6.10 | 각 신규 체크 스펙+테스트 커버 |

**Common conventions** (모든 task 공통):

- 모든 신규 파일은 package doc comment로 시작 (`// Package X ...`).
- 신규 코드의 export된 심볼은 godoc 주석 필수.
- 테스트 파일은 implementation과 같은 package (`_test.go`).
- Commit prefix: `[γ.N.M]` 형식 (phase 번호 + task 번호). 메시지 본문에 *왜* 한 문단.
- 각 commit 후 `go test ./...`, `go vet ./...` 통과 확인. golangci-lint는 PR/푸시 전에만.
- 라인 ending은 LF 그대로 (Windows 환경에서 Git이 자동 변환 경고는 무시).
- "Run: ..." 의 Expected는 정확한 stdout이 아닌 핵심 시그널만 명시.

---

## Phase γ.1 — Dual-Tick Foundation (W1–2)

목표: `ServerTick` 시계 + `ClientTick` 명시 분리, `data.Player` 위치 3-상태(`ExpectedPos`/`ClaimedPos`/`CommittedPos`), `sim.Step()` 순수 함수, 모든 체크에 `TickContext` 전파. **체크 로직은 변경 금지** — shadow 모드. β 테스트 전부 green이 phase 게이트.

---

### Task T1.1: TickContext 타입 신설

**Files:**
- Create: `anticheat/meta/tick_context.go`
- Test: `anticheat/meta/tick_context_test.go`

**Step 1: Failing test**

```go
// anticheat/meta/tick_context_test.go
package meta

import "testing"

func TestTickContextSkew(t *testing.T) {
    ctx := TickContext{ServerTick: 100, ClientTick: 95}
    if got := ctx.Skew(); got != 5 {
        t.Fatalf("Skew()=%d want 5", got)
    }
}

func TestTickContextSkewNegativeIsZero(t *testing.T) {
    // ClientTick > ServerTick should not happen but must not panic.
    ctx := TickContext{ServerTick: 5, ClientTick: 10}
    if got := ctx.Skew(); got != 0 {
        t.Fatalf("Skew()=%d want 0 (clamped)", got)
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/meta/...`
Expected: FAIL — `tick_context.go` 없음.

**Step 3: Implement**

```go
// anticheat/meta/tick_context.go
package meta

// TickContext carries the dual-tick frame in which a check executes.
// It is constructed once per packet (PlayerAuthInput / UseItemOnEntity / etc.)
// in Manager and threaded through to every Detection.Check call.
type TickContext struct {
    // ServerTick is the proxy's monotonic tick (20 TPS, 50 ms each).
    ServerTick uint64
    // ClientTick is the tick the client claims via PlayerAuthInput.Tick
    // (or the last-seen value for non-input events).
    ClientTick uint64
}

// Skew returns ServerTick - ClientTick clamped to ≥ 0. Negative skew
// (client tick ahead of server) is a Timer/A signal but here we just
// report 0 so downstream rewind math never indexes negative.
func (c TickContext) Skew() uint64 {
    if c.ClientTick > c.ServerTick {
        return 0
    }
    return c.ServerTick - c.ClientTick
}
```

**Step 4: Verify pass**

Run: `go test ./anticheat/meta/...`
Expected: PASS (2 tests).

**Step 5: Commit**

```bash
git add anticheat/meta/tick_context.go anticheat/meta/tick_context_test.go
git commit -m "[γ.1.1] meta: TickContext{ServerTick,ClientTick} primitive

Foundation for dual-tick model. Skew() clamps negative to 0; the negative
case (client claims tick ahead of server) is itself a Timer/A signal
handled elsewhere, so here we just keep rewind math safe."
```

---

### Task T1.2: Manager ServerTick ticker

**Files:**
- Modify: `anticheat/anticheat.go` (Manager struct + NewManager + new methods)
- Test: `anticheat/server_tick_test.go`

**Step 1: Failing test**

```go
// anticheat/server_tick_test.go
package anticheat

import (
    "io"
    "log/slog"
    "testing"
    "time"

    "github.com/boredape874/Better-pm-AC/config"
)

func TestManagerServerTickAdvances(t *testing.T) {
    m := NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
    defer m.Stop()

    m.StartTicker()
    start := m.ServerTick()
    time.Sleep(120 * time.Millisecond) // ≥ 2 ticks at 20 TPS
    end := m.ServerTick()
    if end-start < 2 {
        t.Fatalf("ServerTick advanced %d ticks in 120ms; want ≥2", end-start)
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/ -run TestManagerServerTick -v`
Expected: FAIL — `Stop`, `StartTicker`, `ServerTick` undefined.

**Step 3: Implement**

In `anticheat/anticheat.go` add to Manager struct:

```go
    serverTick uint64
    tickStop   chan struct{}
    tickWG     sync.WaitGroup
```

Add methods at end of file:

```go
// ServerTick returns the current monotonic proxy tick. Read with sync/atomic.
func (m *Manager) ServerTick() uint64 {
    return atomic.LoadUint64(&m.serverTick)
}

// StartTicker boots the 20 TPS goroutine that advances ServerTick. Idempotent.
func (m *Manager) StartTicker() {
    if m.tickStop != nil {
        return
    }
    m.tickStop = make(chan struct{})
    m.tickWG.Add(1)
    go func() {
        defer m.tickWG.Done()
        t := time.NewTicker(50 * time.Millisecond)
        defer t.Stop()
        for {
            select {
            case <-t.C:
                atomic.AddUint64(&m.serverTick, 1)
            case <-m.tickStop:
                return
            }
        }
    }()
}

// Stop halts the ticker. Safe to call multiple times.
func (m *Manager) Stop() {
    if m.tickStop == nil {
        return
    }
    close(m.tickStop)
    m.tickWG.Wait()
    m.tickStop = nil
}
```

Add imports: `"sync/atomic"`, `"time"` (sync already imported).

**Step 4: Verify pass**

Run: `go test ./anticheat/ -run TestManagerServerTick -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS (β tests untouched).

**Step 5: Commit**

```bash
git add anticheat/anticheat.go anticheat/server_tick_test.go
git commit -m "[γ.1.2] manager: ServerTick ticker (20 TPS, atomic)

Introduces the proxy-side monotonic clock. Not yet plumbed into checks —
that's T1.4. Atomic counter so reads are lock-free, ticker goroutine
gated by Stop() for clean shutdown in tests."
```

---

### Task T1.3: Player position 3-state fields

**Files:**
- Modify: `anticheat/data/player.go` (Player struct + accessors)
- Test: `anticheat/data/player_pos_states_test.go`

**Step 1: Failing test**

```go
// anticheat/data/player_pos_states_test.go
package data

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestPlayerCommitPosFlow(t *testing.T) {
    p := NewPlayer(uuid.New(), "u")
    claimed := mgl32.Vec3{1, 64, 0}
    expected := mgl32.Vec3{0.99, 64, 0}

    p.SetClaimedPos(claimed)
    p.SetExpectedPos(expected)
    if p.ClaimedPos() != claimed {
        t.Fatalf("ClaimedPos=%v want %v", p.ClaimedPos(), claimed)
    }
    if p.ExpectedPos() != expected {
        t.Fatalf("ExpectedPos=%v want %v", p.ExpectedPos(), expected)
    }

    p.Commit(claimed)
    if p.CommittedPos() != claimed {
        t.Fatalf("CommittedPos after Commit=%v want %v", p.CommittedPos(), claimed)
    }
    if p.PrevCommittedPos() != (mgl32.Vec3{}) {
        t.Fatalf("first Commit prev=%v want zero", p.PrevCommittedPos())
    }

    p.Commit(mgl32.Vec3{2, 64, 0})
    if p.PrevCommittedPos() != claimed {
        t.Fatalf("second Commit prev=%v want %v", p.PrevCommittedPos(), claimed)
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/data/ -run TestPlayerCommitPosFlow -v`
Expected: FAIL — accessors undefined.

**Step 3: Implement**

In `anticheat/data/player.go` Player struct, add (alongside existing pos fields):

```go
    // Dual-tick authoritative position triplet (γ.1+).
    // claimedPos: from PlayerAuthInput.Position (unvalidated).
    // expectedPos: sim.Step() output (truth).
    // committedPos: per-tick accepted final pos; next tick's prev base.
    claimedPos      mgl32.Vec3
    expectedPos     mgl32.Vec3
    committedPos    mgl32.Vec3
    prevCommittedPos mgl32.Vec3
```

Add methods (anywhere in file, near existing position accessors):

```go
// SetClaimedPos records the client-claimed position from PlayerAuthInput.
// Caller is the Manager; checks must consume CommittedPos() instead.
func (p *Player) SetClaimedPos(v mgl32.Vec3) { p.claimedPos = v }

// ClaimedPos returns the most-recent client-claimed position.
func (p *Player) ClaimedPos() mgl32.Vec3 { return p.claimedPos }

// SetExpectedPos records the sim-computed expected position. Truth.
func (p *Player) SetExpectedPos(v mgl32.Vec3) { p.expectedPos = v }

// ExpectedPos returns the sim-computed expected position for the current tick.
func (p *Player) ExpectedPos() mgl32.Vec3 { return p.expectedPos }

// Commit promotes a position to the per-tick accepted state, rolling the
// previous committed pos into prevCommittedPos. Called by reconciliation
// once per tick, never directly by checks.
func (p *Player) Commit(v mgl32.Vec3) {
    p.prevCommittedPos = p.committedPos
    p.committedPos = v
}

// CommittedPos is the per-tick accepted final position. Checks read this.
func (p *Player) CommittedPos() mgl32.Vec3 { return p.committedPos }

// PrevCommittedPos is the previous tick's committed position. Used for delta math.
func (p *Player) PrevCommittedPos() mgl32.Vec3 { return p.prevCommittedPos }
```

**Step 4: Verify pass**

Run: `go test ./anticheat/data/ -run TestPlayerCommitPosFlow -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/data/player.go anticheat/data/player_pos_states_test.go
git commit -m "[γ.1.3] data.Player: Claimed/Expected/Committed pos triplet

Backbone of the authoritative model. Existing UpdatePosition path stays
unchanged so β checks keep working — γ.3 will migrate them to read from
CommittedPos. First Commit() leaves prev as zero (no previous frame yet)."
```

---

### Task T1.4: Manager OnInput threads ServerTick + claimed pos

**Files:**
- Modify: `anticheat/anticheat.go` (OnInput)
- Test: `anticheat/oninput_dualtick_test.go`

**Step 1: Failing test**

```go
// anticheat/oninput_dualtick_test.go
package anticheat

import (
    "io"
    "log/slog"
    "testing"

    "github.com/boredape874/Better-pm-AC/config"
    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

type emptyBitset struct{}

func (emptyBitset) Load(int) bool { return false }

func TestOnInputRecordsClaimedPosAndAdvancesTick(t *testing.T) {
    m := NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
    id := uuid.New()
    m.AddPlayer(id, "u")

    pos := mgl32.Vec3{1, 64, 0}
    m.OnInput(id, 5, pos, true, 0, 0, 1, emptyBitset{})

    p := m.GetPlayer(id)
    if p.ClaimedPos() != pos {
        t.Fatalf("ClaimedPos=%v want %v", p.ClaimedPos(), pos)
    }
    if p.LastClientTick() != 5 {
        t.Fatalf("LastClientTick=%d want 5", p.LastClientTick())
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/ -run TestOnInputRecordsClaimed -v`
Expected: FAIL.

**Step 3: Implement**

In `anticheat/data/player.go` add:

```go
    lastClientTick uint64
```
to Player struct, plus:

```go
// LastClientTick returns the last PlayerAuthInput.Tick observed.
func (p *Player) LastClientTick() uint64 { return p.lastClientTick }

// SetLastClientTick records the client tick.
func (p *Player) SetLastClientTick(t uint64) { p.lastClientTick = t }
```

In `anticheat/anticheat.go` `OnInput`, immediately after the early-return nil checks, add:

```go
    p.SetClaimedPos(pos)
    p.SetLastClientTick(tick)
```

(Existing `p.UpdateTick(tick)` / `p.UpdatePosition(pos, onGround)` calls stay — γ.3 will reshape them.)

**Step 4: Verify pass**

Run: `go test ./anticheat/ -run TestOnInputRecordsClaimed -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/anticheat.go anticheat/data/player.go anticheat/oninput_dualtick_test.go
git commit -m "[γ.1.4] OnInput: thread claimed pos + last client tick into Player

Pure additive: existing UpdatePosition still runs, β checks unaffected.
Sets up γ.2 reconciliation to find ClaimedPos in a known place."
```

---

### Task T1.5: sim.Step() pure-function shape

**Files:**
- Modify: `anticheat/sim/engine.go` (extract pure Step)
- Test: `anticheat/sim/step_test.go`

**Step 1: Failing test**

```go
// anticheat/sim/step_test.go
package sim

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
)

func TestStepGravityFromAir(t *testing.T) {
    in := StepInput{
        PrevPos:  mgl32.Vec3{0, 64, 0},
        Velocity: mgl32.Vec3{0, 0, 0},
        OnGround: false,
    }
    out := Step(in)
    if out.Velocity.Y() >= 0 {
        t.Fatalf("Step from air should apply gravity, got vy=%f", out.Velocity.Y())
    }
}

func TestStepIsPure(t *testing.T) {
    in := StepInput{
        PrevPos:  mgl32.Vec3{0, 64, 0},
        Velocity: mgl32.Vec3{0.1, 0, 0},
    }
    a := Step(in)
    b := Step(in)
    if a != b {
        t.Fatalf("Step is not pure: %+v vs %+v", a, b)
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/sim/ -run TestStep -v`
Expected: FAIL — `Step`, `StepInput`, `StepOutput` undefined.

**Step 3: Implement**

In `anticheat/sim/engine.go` add at end:

```go
// StepInput is the immutable input to the deterministic single-tick simulator.
// All physics state needed for one tick must arrive in this struct — Step()
// must not read any package-level globals. This is what makes property-based
// testing of the sim viable.
type StepInput struct {
    PrevPos       mgl32.Vec3
    Velocity      mgl32.Vec3
    OnGround      bool
    Effects       Effects // existing struct from effects.go
    InputForward  float32
    InputStrafe   float32
    Sprint        bool
    Sneak         bool
    Jump          bool
    InLiquid      bool
    OnClimbable   bool
}

// StepOutput is the result of one simulation tick.
type StepOutput struct {
    ExpectedPos mgl32.Vec3
    Velocity    mgl32.Vec3
}

// Step advances the simulation by exactly one 50 ms tick using the rules
// already encoded in this package (gravity, friction, jump, walk, fluid).
// It is pure: same input → same output, no shared state.
//
// γ.1 stub: assembles existing helpers without world / collision context.
// γ.5 will add small-Δ collision and lava-specific drag here. The current
// shape is sufficient for shadow-mode plumbing.
func Step(in StepInput) StepOutput {
    v := in.Velocity
    if !in.OnGround {
        v = applyGravity(v, in.Effects)
    }
    if in.Jump && in.OnGround {
        v = applyJump(v, in.Effects)
    }
    v = applyWalk(v, in.InputForward, in.InputStrafe, in.Sprint, in.Sneak, in.OnGround, in.Effects)
    v = applyFriction(v, in.OnGround)
    pos := in.PrevPos.Add(v)
    return StepOutput{ExpectedPos: pos, Velocity: v}
}
```

The helper names (`applyGravity`, `applyJump`, `applyWalk`, `applyFriction`) must match what already exists in this package. If they have slightly different signatures (e.g. take `*State` instead of `Vec3`), adapt by extracting the velocity-only inner core into private helpers `gravityVelOnly`, `jumpVelOnly`, etc., and call those from Step. The point is Step takes no pointers to mutable state.

**Step 4: Verify pass**

Run: `go test ./anticheat/sim/ -run TestStep -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/sim/engine.go anticheat/sim/step_test.go
git commit -m "[γ.1.5] sim: pure Step(StepInput) StepOutput facade

Shadow shape that wraps existing helpers without taking mutable State
pointers. γ.5 fills in collision and small-Δ resolver here. The purity
test is the contract: it locks the door against future drift back to
hidden globals."
```

---

### Task T1.6: Detection interface gains TickContext (Authority enum)

**Files:**
- Modify: `anticheat/meta/detection.go`
- Test: `anticheat/meta/authority_test.go`

**Step 1: Failing test**

```go
// anticheat/meta/authority_test.go
package meta

import "testing"

func TestAuthorityZeroIsValidate(t *testing.T) {
    var a Authority
    if a != AuthorityValidate {
        t.Fatalf("zero Authority=%v want %v", a, AuthorityValidate)
    }
}

func TestAuthorityStringStable(t *testing.T) {
    cases := map[Authority]string{
        AuthorityValidate:             "validate",
        AuthorityAuthoritativeMovement: "authoritative_movement",
        AuthorityAuthoritativeCombat:  "authoritative_combat",
    }
    for a, want := range cases {
        if a.String() != want {
            t.Fatalf("Authority(%d).String()=%q want %q", a, a.String(), want)
        }
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/meta/ -run TestAuthority -v`
Expected: FAIL.

**Step 3: Implement**

In `anticheat/meta/detection.go` append:

```go
// Authority describes how a Detection participates in the dual-tick model.
// Validate (zero value) is a passive observer reading CommittedPos.
// AuthoritativeMovement / AuthoritativeCombat declare that the check
// drives mitigation snaps via mitigate.Dispatcher.
type Authority int

const (
    AuthorityValidate Authority = iota
    AuthorityAuthoritativeMovement
    AuthorityAuthoritativeCombat
)

func (a Authority) String() string {
    switch a {
    case AuthorityAuthoritativeMovement:
        return "authoritative_movement"
    case AuthorityAuthoritativeCombat:
        return "authoritative_combat"
    default:
        return "validate"
    }
}
```

We do **not** add `Authority()` to the Detection interface yet — that would force every check to declare it in this commit. γ.3 / γ.4 will migrate checks one at a time.

**Step 4: Verify pass**

Run: `go test ./anticheat/meta/ -run TestAuthority -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add anticheat/meta/detection.go anticheat/meta/authority_test.go
git commit -m "[γ.1.6] meta: Authority enum (Validate/AuthMovement/AuthCombat)

Lays the type without modifying the Detection interface so β checks stay
binary-compatible. γ.3/.4 will migrate checks one by one and add the
interface method then."
```

---

### Task T1.7: Manager constructs TickContext per OnInput

**Files:**
- Modify: `anticheat/anticheat.go` (OnInput)
- Test: `anticheat/oninput_tickctx_test.go`

**Step 1: Failing test**

```go
// anticheat/oninput_tickctx_test.go
package anticheat

import (
    "io"
    "log/slog"
    "testing"

    "github.com/boredape874/Better-pm-AC/config"
    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestOnInputBuildsTickContext(t *testing.T) {
    m := NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
    id := uuid.New()
    m.AddPlayer(id, "u")

    // Force a known ServerTick.
    m.SetServerTickForTest(100)

    m.OnInput(id, 95, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, emptyBitset{})

    ctx := m.LastTickContextForTest(id)
    if ctx.ServerTick != 100 || ctx.ClientTick != 95 {
        t.Fatalf("ctx=%+v want {100,95}", ctx)
    }
    if ctx.Skew() != 5 {
        t.Fatalf("skew=%d want 5", ctx.Skew())
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/ -run TestOnInputBuildsTickContext -v`
Expected: FAIL.

**Step 3: Implement**

In `anticheat/anticheat.go`:

1. Add to Manager struct:
   ```go
       lastTickCtx map[uuid.UUID]meta.TickContext // test-only mirror
   ```
   Initialize in NewManager: `lastTickCtx: make(map[uuid.UUID]meta.TickContext)`.

2. At the top of `OnInput`, after the nil checks, replace existing tick handling:

   ```go
       sTick := atomic.LoadUint64(&m.serverTick)
       ctx := meta.TickContext{ServerTick: sTick, ClientTick: tick}
       p.SetClaimedPos(pos)
       p.SetLastClientTick(tick)
       m.mu.Lock()
       m.lastTickCtx[id] = ctx
       m.mu.Unlock()
   ```

   (Keep the existing `p.UpdateTick(tick)` call.)

3. Add at end of file:

   ```go
   // SetServerTickForTest forces the ServerTick counter (test helper only).
   func (m *Manager) SetServerTickForTest(v uint64) {
       atomic.StoreUint64(&m.serverTick, v)
   }

   // LastTickContextForTest returns the most recent TickContext for a player.
   func (m *Manager) LastTickContextForTest(id uuid.UUID) meta.TickContext {
       m.mu.RLock()
       defer m.mu.RUnlock()
       return m.lastTickCtx[id]
   }
   ```

**Step 4: Verify pass**

Run: `go test ./anticheat/ -run TestOnInputBuildsTickContext -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/anticheat.go anticheat/oninput_tickctx_test.go
git commit -m "[γ.1.7] OnInput: build TickContext{ServerTick,ClientTick}

Test-only mirror so γ.3 has something to assert against once checks
start consuming TickContext. Production path is identical to before."
```

---

### Task T1.8: AckSystem key extension to {ServerTick, ClientTick}

**Files:**
- Modify: `anticheat/ack/system.go` (key type, Resolve signature)
- Modify: `anticheat/ack/marker.go` if marker keys use the old shape
- Test: `anticheat/ack/dualkey_test.go`

**Step 1: Failing test**

```go
// anticheat/ack/dualkey_test.go
package ack

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
)

func TestPushAndResolveByDualKey(t *testing.T) {
    s := NewSystem()
    s.Push(Key{ServerTick: 100, ClientTick: 95}, Action{Kind: ActionKnockback, ExpectedDelta: mgl32.Vec3{0.4, 0, 0}})

    matched, act := s.Resolve(Key{ServerTick: 100, ClientTick: 95}, mgl32.Vec3{0.41, 0, 0})
    if !matched {
        t.Fatalf("expected match for similar Δ")
    }
    if act.Kind != ActionKnockback {
        t.Fatalf("act.Kind=%v want %v", act.Kind, ActionKnockback)
    }
}

func TestResolveMismatchOnLargeDelta(t *testing.T) {
    s := NewSystem()
    s.Push(Key{ServerTick: 100, ClientTick: 95}, Action{ExpectedDelta: mgl32.Vec3{0.4, 0, 0}})
    matched, _ := s.Resolve(Key{ServerTick: 100, ClientTick: 95}, mgl32.Vec3{2.0, 0, 0})
    if matched {
        t.Fatalf("expected mismatch on Δ=2.0 vs expected 0.4")
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/ack/ -run TestPushAndResolve -v`
Expected: FAIL.

**Step 3: Implement**

Replace `system.go` core types with:

```go
// Key uniquely identifies a pending action by both clocks.
type Key struct {
    ServerTick uint64
    ClientTick uint64
}

// ActionKind is the category of pending action awaiting client ack.
type ActionKind int

const (
    ActionUnknown ActionKind = iota
    ActionKnockback
    ActionTeleport
    ActionCorrection
)

// Action is a single pending ack entry.
type Action struct {
    Kind          ActionKind
    ExpectedDelta mgl32.Vec3 // sim-predicted Δpos for this action
}

// System is the proxy-side ack queue. Concurrency-safe.
type System struct {
    mu      sync.Mutex
    pending map[Key]Action
}

func NewSystem() *System {
    return &System{pending: make(map[Key]Action)}
}

// Push registers a pending action. Replaces any prior entry at the same Key.
func (s *System) Push(k Key, a Action) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.pending[k] = a
}

// Resolve matches the actual observed Δpos against the pending action at Key.
// Returns (matched=true, action) when |actualΔ - expected| ≤ matchTol.
// On match the action is removed from the queue.
const matchTol = 0.15

func (s *System) Resolve(k Key, actualDelta mgl32.Vec3) (bool, Action) {
    s.mu.Lock()
    defer s.mu.Unlock()
    a, ok := s.pending[k]
    if !ok {
        return false, Action{}
    }
    diff := actualDelta.Sub(a.ExpectedDelta).Len()
    if diff > matchTol {
        return false, Action{}
    }
    delete(s.pending, k)
    return true, a
}
```

Imports: `"sync"`, `"github.com/go-gl/mathgl/mgl32"`.

If `marker.go` references the old single-tick key, update its callers to construct `Key{ClientTick: t}` (ServerTick 0 is fine for legacy markers — γ.2 will reach in and set ServerTick where it matters).

**Step 4: Verify pass**

Run: `go test ./anticheat/ack/ -v`
Expected: PASS (existing system_test.go may need adjustment to new shape — bring it forward by replacing old keys with `Key{ClientTick: t}`).

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/ack/
git commit -m "[γ.1.8] ack: dual-clock Key + ExpectedDelta + Resolve(Δ)

Reconciliation needs to ask 'is this Δpos consistent with a server-issued
action that was pending?'. Push records the prediction; Resolve matches
within 0.15 b. The constant gets re-tuned in γ.5 against PMMP traffic."
```

---

### Task T1.9: Wire Manager.StartTicker() into proxy startup

**Files:**
- Modify: `proxy/proxy.go` (proxy lifecycle)

**Step 1: Failing test (proxy startup smoke)**

The proxy package likely already has a `TestProxyStarts` style smoke. If not, add a minimal one in `proxy/proxy_lifecycle_test.go`:

```go
package proxy

import "testing"

func TestProxyStartsManagerTicker(t *testing.T) {
    p := newTestProxy(t) // existing test helper or build one inline
    if p.Anticheat.ServerTick() != 0 {
        t.Fatalf("ServerTick before start = %d, want 0", p.Anticheat.ServerTick())
    }
    p.Start()
    defer p.Stop()
    waitForTickAdvance(t, p.Anticheat, 2, time.Second)
}
```

If a test helper doesn't exist, defer this to the existing `proxy_test.go` and just confirm by inspection that `Start()` calls `StartTicker()`.

**Step 2: Verify fail**

Run: `go test ./proxy/ -v`
Expected: FAIL on the new test.

**Step 3: Implement**

In `proxy/proxy.go`, in the proxy struct's `Start()` (or wherever it boots resources) add:

```go
    p.Anticheat.StartTicker()
```

In the corresponding `Stop()`/cleanup add `p.Anticheat.Stop()`.

**Step 4: Verify pass**

Run: `go test ./proxy/ -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add proxy/proxy.go proxy/proxy_lifecycle_test.go
git commit -m "[γ.1.9] proxy: start/stop the Manager ServerTick ticker

Hooks the 20 TPS ticker into proxy lifecycle. Without this the ticker
goroutine never runs in production and TickContext.ServerTick stays at
0 — silent γ.2 reconciliation bug if missed."
```

---

### Task T1.10: γ.1 phase gate — full β regression run

**Files:** none (verification commit).

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: ALL PASS.

**Step 2: Run vet**

Run: `go vet ./...`
Expected: no diagnostics.

**Step 3: Run lint locally** (CI runs this too)

Run: `golangci-lint run ./...` (if installed; otherwise document as CI-deferred)
Expected: no errors.

**Step 4: Run benchmarks for regression baseline**

Run: `go test -bench=. -benchtime=1s -run=^$ ./bench/...`
Expected: roughly within 20% of β baseline (≤ 10 µs/player on `BenchmarkOnInput100CCU`).

**Step 5: Tag the phase end**

```bash
git tag γ.1-foundation
git log --oneline γ.1-foundation~10..γ.1-foundation
```

Document baseline numbers in `docs/plans/2026-04-19-work-board.md` under a new "γ.1 done" line including bench output.

**Step 6: Commit work-board update**

```bash
git add docs/plans/2026-04-19-work-board.md
git commit -m "[γ.1.10] board: γ.1 done; bench baseline recorded"
```

---

## Phase γ.2 — Reconciliation + Authoritative Correction (W2–3)

목표: §2.3의 3-분기 reconcile (accept / pending / snap), `CorrectPlayerMovePrediction` 송신 채널, upstream `PlayerAuthInput` rewrite. Feature flag 도입. 게이트: legit scenario 10개 FP=0.

---

### Task T2.1: Authority feature-flag config block

**Files:**
- Modify: `config/config.go` (add `[anticheat.authority]` block)
- Test: `config/authority_default_test.go`

**Step 1: Failing test**

```go
// config/authority_default_test.go
package config

import "testing"

func TestAuthorityDefaultsAllOff(t *testing.T) {
    cfg := Default().Anticheat.Authority
    if cfg.DualTick || cfg.ReconcileSnap || cfg.UpstreamRewrite ||
        cfg.MovementAuth || cfg.CombatMultiRay {
        t.Fatalf("γ.2 phase: all authority flags must default off, got %+v", cfg)
    }
}
```

**Step 2: Verify fail**

Run: `go test ./config/ -run TestAuthorityDefaults -v`
Expected: FAIL.

**Step 3: Implement**

In `config/config.go` add:

```go
// AuthorityConfig flags the dual-tick / authoritative-movement features.
// All default off so γ.2 ships in shadow mode and operators canary explicitly.
type AuthorityConfig struct {
    DualTick        bool `toml:"dual_tick"`
    ReconcileSnap   bool `toml:"reconcile_snap"`
    UpstreamRewrite bool `toml:"upstream_rewrite"`
    MovementAuth    bool `toml:"movement_auth"`
    CombatMultiRay  bool `toml:"combat_multiray"`
}
```

In `AnticheatConfig` struct add: `Authority AuthorityConfig` with TOML tag `toml:"authority"`. In `Default()` no override needed (zero value = all off).

**Step 4: Verify pass**

Run: `go test ./config/ -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add config/config.go config/authority_default_test.go
git commit -m "[γ.2.1] config: AuthorityConfig with five feature flags

Every γ behavior gates on one of these. Defaults all-off so γ.2/3/4
ships in shadow mode by default; operators flip them on per canary."
```

---

### Task T2.2: Reconciler core (3-branch decision, no side effects)

**Files:**
- Create: `anticheat/reconcile/reconciler.go`
- Create: `anticheat/reconcile/doc.go`
- Test: `anticheat/reconcile/reconciler_test.go`

**Step 1: Failing test**

```go
// anticheat/reconcile/reconciler_test.go
package reconcile

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
)

func TestDecideAcceptWithinEpsAccept(t *testing.T) {
    d := Decide(Input{
        ClaimedPos:  mgl32.Vec3{0, 64, 0},
        ExpectedPos: mgl32.Vec3{0.02, 64, 0},
    })
    if d.Outcome != OutcomeAccept {
        t.Fatalf("outcome=%v want Accept", d.Outcome)
    }
}

func TestDecidePendingWithinEpsPending(t *testing.T) {
    d := Decide(Input{
        ClaimedPos:  mgl32.Vec3{0, 64, 0},
        ExpectedPos: mgl32.Vec3{0.2, 64, 0},
    })
    if d.Outcome != OutcomePending {
        t.Fatalf("outcome=%v want Pending", d.Outcome)
    }
}

func TestDecideSnapBeyondEpsPending(t *testing.T) {
    d := Decide(Input{
        ClaimedPos:  mgl32.Vec3{0, 64, 0},
        ExpectedPos: mgl32.Vec3{1.0, 64, 0},
    })
    if d.Outcome != OutcomeSnap {
        t.Fatalf("outcome=%v want Snap", d.Outcome)
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/reconcile/ -v`
Expected: FAIL — package missing.

**Step 3: Implement**

```go
// anticheat/reconcile/doc.go
// Package reconcile decides per-tick whether a client-claimed position is
// accepted, awaiting ack, or must be snapped to the sim-expected position.
// It is a pure decision function — side effects (snap packet send, Player
// state mutation) belong to the Manager, which calls Decide and acts on
// the result.
package reconcile
```

```go
// anticheat/reconcile/reconciler.go
package reconcile

import "github.com/go-gl/mathgl/mgl32"

// Default tolerance bands. γ.5 may re-tune against PMMP traffic.
const (
    EpsAccept  = 0.03
    EpsPending = 0.30
)

// Outcome of a reconciliation decision.
type Outcome int

const (
    OutcomeAccept  Outcome = iota // |Δ| ≤ EpsAccept
    OutcomePending                 // EpsAccept < |Δ| ≤ EpsPending; check ack queue
    OutcomeSnap                    // |Δ| > EpsPending; force ExpectedPos
)

func (o Outcome) String() string {
    switch o {
    case OutcomeAccept:
        return "accept"
    case OutcomePending:
        return "pending"
    case OutcomeSnap:
        return "snap"
    default:
        return "?"
    }
}

// Input captures what Decide needs.
type Input struct {
    ClaimedPos  mgl32.Vec3
    ExpectedPos mgl32.Vec3
}

// Decision is the result.
type Decision struct {
    Outcome  Outcome
    DeltaLen float32
}

// Decide is pure: same input → same output.
func Decide(in Input) Decision {
    d := in.ClaimedPos.Sub(in.ExpectedPos).Len()
    switch {
    case d <= EpsAccept:
        return Decision{Outcome: OutcomeAccept, DeltaLen: d}
    case d <= EpsPending:
        return Decision{Outcome: OutcomePending, DeltaLen: d}
    default:
        return Decision{Outcome: OutcomeSnap, DeltaLen: d}
    }
}
```

**Step 4: Verify pass**

Run: `go test ./anticheat/reconcile/ -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add anticheat/reconcile/
git commit -m "[γ.2.2] reconcile: pure 3-branch Decide(Claimed,Expected)

Two thresholds, three outcomes. No side effects so the Manager can
unit-test reconciliation paths cheaply and we can swap thresholds
without touching call sites."
```

---

### Task T2.3: Manager runs sim → reconcile → commit (flagged)

**Files:**
- Modify: `anticheat/anticheat.go`
- Test: `anticheat/oninput_reconcile_test.go`

**Step 1: Failing test**

```go
// anticheat/oninput_reconcile_test.go
package anticheat

import (
    "io"
    "log/slog"
    "testing"

    "github.com/boredape874/Better-pm-AC/config"
    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestReconcileAcceptsClaimedWhenFlagOn(t *testing.T) {
    cfg := config.Default().Anticheat
    cfg.Authority.DualTick = true
    cfg.Authority.ReconcileSnap = true

    m := NewManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
    id := uuid.New()
    m.AddPlayer(id, "u")
    m.SetServerTickForTest(2)

    // Seed prev-committed pos by feeding tick 1 first so claimed≈expected.
    m.OnInput(id, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, emptyBitset{})
    p := m.GetPlayer(id)
    if p.CommittedPos() != (mgl32.Vec3{0, 64, 0}) {
        t.Fatalf("first tick CommittedPos=%v want origin", p.CommittedPos())
    }
}

func TestReconcileShadowModeDoesNotCommitWhenFlagOff(t *testing.T) {
    cfg := config.Default().Anticheat // all flags off
    m := NewManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
    id := uuid.New()
    m.AddPlayer(id, "u")

    m.OnInput(id, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, emptyBitset{})
    p := m.GetPlayer(id)
    if p.CommittedPos() != (mgl32.Vec3{}) {
        t.Fatalf("flag off: CommittedPos must stay zero, got %v", p.CommittedPos())
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/ -run TestReconcile -v`
Expected: FAIL.

**Step 3: Implement**

In `anticheat/anticheat.go` `OnInput`, after `p.SetClaimedPos(pos)` and BEFORE existing β check execution, add:

```go
    if m.cfg.Authority.DualTick {
        // Build sim input from current Player state.
        in := sim.StepInput{
            PrevPos:  p.PrevCommittedPos(),
            // γ.1 sim takes only the velocity-relevant inputs; effects/world
            // come in γ.5 when sim becomes truly authoritative. For now this
            // stub mirrors the pre-existing in-Manager calculation.
            OnGround: onGround,
        }
        out := sim.Step(in)
        p.SetExpectedPos(out.ExpectedPos)

        if m.cfg.Authority.ReconcileSnap {
            d := reconcile.Decide(reconcile.Input{
                ClaimedPos:  pos,
                ExpectedPos: out.ExpectedPos,
            })
            switch d.Outcome {
            case reconcile.OutcomeAccept:
                p.Commit(pos)
            case reconcile.OutcomePending:
                // T2.5 ties this into the ack queue; for now treat as accept.
                p.Commit(pos)
            case reconcile.OutcomeSnap:
                // T2.6 emits CorrectPlayerMovePrediction here.
                p.Commit(out.ExpectedPos)
            }
            mitigate.RecordReconcile(d.Outcome.String())
        } else {
            p.Commit(pos) // shadow: flag off → accept everything
        }
    }
```

Add imports: `"github.com/boredape874/Better-pm-AC/anticheat/reconcile"`, `"github.com/boredape874/Better-pm-AC/anticheat/sim"`.

`mitigate.RecordReconcile` is forward-referenced — T2.7 implements it. For now stub it as a no-op in `mitigate/metrics.go`.

**Step 4: Verify pass**

Run: `go test ./anticheat/ -run TestReconcile -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS (β behavior unchanged; flags default off).

**Step 5: Commit**

```bash
git add anticheat/anticheat.go anticheat/mitigate/metrics.go anticheat/oninput_reconcile_test.go
git commit -m "[γ.2.3] manager: sim→reconcile→commit pipeline behind DualTick flag

Off by default so β still wins production. With both flags on, claimed
poses get accepted/snap-decisions made before checks run — but checks
themselves are still β shape. T3 migrates checks to read CommittedPos."
```

---

### Task T2.4: mitigate.CorrectFunc + dispatcher integration

**Files:**
- Modify: `anticheat/mitigate/policy.go`
- Test: `anticheat/mitigate/correct_test.go`

**Step 1: Failing test**

```go
// anticheat/mitigate/correct_test.go
package mitigate

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestCorrectFuncInvokedOnSnap(t *testing.T) {
    var called bool
    var gotPos mgl32.Vec3
    var gotTick uint64

    cf := CorrectFunc(func(_ uuid.UUID, p mgl32.Vec3, tick uint64) {
        called = true
        gotPos = p
        gotTick = tick
    })

    cf(uuid.Nil, mgl32.Vec3{1, 64, 0}, 100)
    if !called || gotPos != (mgl32.Vec3{1, 64, 0}) || gotTick != 100 {
        t.Fatalf("CorrectFunc not invoked correctly")
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/mitigate/ -run TestCorrectFunc -v`
Expected: FAIL.

**Step 3: Implement**

In `anticheat/mitigate/policy.go` add:

```go
// CorrectFunc is the high-confidence authoritative correction channel,
// distinct from RubberbandFunc which stays as the low-confidence
// fallback (used for legacy clients and unclear sim divergences).
// Implementations send a CorrectPlayerMovePrediction packet downstream.
type CorrectFunc func(id uuid.UUID, expected mgl32.Vec3, tick uint64)
```

(Actual call site wiring lands in T2.6.)

**Step 4: Verify pass**

Run: `go test ./anticheat/mitigate/ -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add anticheat/mitigate/policy.go anticheat/mitigate/correct_test.go
git commit -m "[γ.2.4] mitigate: CorrectFunc type for authoritative snap channel

Distinct from RubberbandFunc so we can keep low-confidence fallback
alive for legacy/vehicle paths without conflating call semantics."
```

---

### Task T2.5: Reconcile pending → ack-queue lookup

**Files:**
- Modify: `anticheat/anticheat.go` (extend reconcile branch)
- Modify: `anticheat/data/player.go` (expose PrevCommittedPos delta)
- Test: `anticheat/oninput_reconcile_pending_test.go`

**Step 1: Failing test**

```go
// anticheat/oninput_reconcile_pending_test.go
package anticheat

import (
    "io"
    "log/slog"
    "testing"

    "github.com/boredape874/Better-pm-AC/anticheat/ack"
    "github.com/boredape874/Better-pm-AC/config"
    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestReconcilePendingAcceptsWhenAckMatches(t *testing.T) {
    cfg := config.Default().Anticheat
    cfg.Authority.DualTick = true
    cfg.Authority.ReconcileSnap = true

    m := NewManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
    id := uuid.New()
    m.AddPlayer(id, "u")
    m.SetServerTickForTest(10)

    // Server pushed a knockback last tick: expects 0.4 b push.
    m.AckSystemForTest(id).Push(
        ack.Key{ServerTick: 10, ClientTick: 5},
        ack.Action{Kind: ActionKnockback, ExpectedDelta: mgl32.Vec3{0.4, 0, 0}},
    )

    // Client now claims a position offset by ~0.4 b — within EpsPending.
    m.OnInput(id, 5, mgl32.Vec3{0.4, 64, 0}, true, 0, 0, 1, emptyBitset{})
    p := m.GetPlayer(id)
    if p.CommittedPos() != (mgl32.Vec3{0.4, 64, 0}) {
        t.Fatalf("matched-ack pending should commit claimed; got %v", p.CommittedPos())
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/ -run TestReconcilePending -v`
Expected: FAIL — `AckSystemForTest`, `ActionKnockback` undefined in this scope.

**Step 3: Implement**

1. Add Manager field `ack *ack.System` and init in NewManager.
2. Add `AckSystemForTest(id uuid.UUID) *ack.System { return m.ack }`.
3. Re-export `ActionKnockback = ack.ActionKnockback` near the keySpeed constants.
4. In OnInput's `OutcomePending` branch, replace the "treat as accept" stub with:

```go
            case reconcile.OutcomePending:
                actualDelta := pos.Sub(p.PrevCommittedPos())
                k := ack.Key{ServerTick: ctx.ServerTick, ClientTick: tick}
                if matched, _ := m.ack.Resolve(k, actualDelta); matched {
                    p.Commit(pos)
                } else {
                    // Pending divergence with no matching ack — treat as accept
                    // for now but the next tick's reconcile will likely re-fire.
                    // γ.3 movement checks consume this signal via Player flag.
                    p.Commit(pos)
                    p.MarkPendingDivergence(ctx.ServerTick)
                }
```

5. Add `MarkPendingDivergence`/`PendingDivergenceTick` methods to Player.

**Step 4: Verify pass**

Run: `go test ./anticheat/ -run TestReconcilePending -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/anticheat.go anticheat/data/player.go anticheat/oninput_reconcile_pending_test.go
git commit -m "[γ.2.5] reconcile: pending branch resolves against ack queue

Knockback / teleport / correction events get pushed into the ack queue
when emitted, then the next tick's reconcile pulls them back to
distinguish 'large Δ but legitimate' from 'large Δ and suspicious'."
```

---

### Task T2.6: proxy emits CorrectPlayerMovePrediction on Snap

**Files:**
- Modify: `proxy/proxy.go` (add CorrectFunc, wire into Manager)
- Test: `proxy/correct_test.go` (recorder hook smoke)

**Step 1: Failing test**

```go
// proxy/correct_test.go
package proxy

// Smoke test: when Manager calls CorrectFunc with a position and tick, the
// proxy must construct a packet.CorrectPlayerMovePrediction and forward it
// to the captured client conn. We verify by stubbing the conn write.
import (
    "testing"
    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestCorrectFuncSendsPacket(t *testing.T) {
    p := newTestProxy(t)
    rec := p.RecordClientPacketsForTest()
    sess := p.AddTestSession(uuid.New())

    p.Anticheat.CorrectFunc(sess.PlayerID, mgl32.Vec3{1, 64, 0}, 100)
    if !rec.Contains("CorrectPlayerMovePrediction") {
        t.Fatalf("no CorrectPlayerMovePrediction sent; got %v", rec.Names())
    }
}
```

(`newTestProxy`, `RecordClientPacketsForTest`, `AddTestSession` are existing/expected proxy test helpers — if missing, scaffold them.)

**Step 2: Verify fail**

Run: `go test ./proxy/ -run TestCorrectFunc -v`
Expected: FAIL.

**Step 3: Implement**

In `proxy/proxy.go`, in the proxy struct add a method:

```go
func (p *Proxy) sendCorrectPrediction(playerID uuid.UUID, expected mgl32.Vec3, tick uint64) {
    sess := p.sessionByPlayerID(playerID)
    if sess == nil || sess.client == nil {
        return
    }
    pk := &packet.CorrectPlayerMovePrediction{
        PredictionType: packet.PredictionTypePlayer,
        Position:       expected,
        Delta:          mgl32.Vec3{}, // sim-clean reset
        OnGround:       true,         // approximation; sim will refine
        Tick:           tick,
    }
    if err := sess.client.WritePacket(pk); err != nil {
        p.log.Warn("CorrectPlayerMovePrediction write failed", "err", err, "uuid", playerID)
    }
}
```

In `Start()` (right after `Anticheat.StartTicker()`):

```go
    p.Anticheat.CorrectFunc = p.sendCorrectPrediction
```

In `anticheat.Manager` add field `CorrectFunc mitigate.CorrectFunc` and call it from the OutcomeSnap branch in OnInput (replacing the inline "T2.6 emits" stub):

```go
            case reconcile.OutcomeSnap:
                p.Commit(out.ExpectedPos)
                if m.CorrectFunc != nil {
                    m.CorrectFunc(p.UUID, out.ExpectedPos, ctx.ServerTick)
                }
```

If `packet.CorrectPlayerMovePrediction` field names differ in the gophertunnel version we have, look up `vendor` or run `go doc github.com/sandertv/gophertunnel/minecraft/protocol/packet CorrectPlayerMovePrediction` and adjust accordingly.

**Step 4: Verify pass**

Run: `go test ./proxy/ -run TestCorrectFunc -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add proxy/proxy.go proxy/correct_test.go anticheat/anticheat.go
git commit -m "[γ.2.6] proxy: send CorrectPlayerMovePrediction on snap

Connects the reconcile Snap outcome to the wire. PredictionType=Player
so vehicle/elytra paths (which need different handling) don't accidentally
get player-shape corrections."
```

---

### Task T2.7: per-check + reconcile metrics in expvar

**Files:**
- Modify: `anticheat/mitigate/metrics.go`
- Test: `anticheat/mitigate/metrics_reconcile_test.go`

**Step 1: Failing test**

```go
// anticheat/mitigate/metrics_reconcile_test.go
package mitigate

import (
    "expvar"
    "testing"
)

func TestRecordReconcileBumpsExpvarCounter(t *testing.T) {
    before := readMapInt("better_pm_ac_reconcile_outcomes", "snap")
    RecordReconcile("snap")
    after := readMapInt("better_pm_ac_reconcile_outcomes", "snap")
    if after-before != 1 {
        t.Fatalf("snap counter delta=%d want 1", after-before)
    }
}

func readMapInt(name, key string) int64 {
    v := expvar.Get(name)
    m, ok := v.(*expvar.Map)
    if !ok {
        return 0
    }
    iv, _ := m.Get(key).(*expvar.Int)
    if iv == nil {
        return 0
    }
    return iv.Value()
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/mitigate/ -run TestRecordReconcile -v`
Expected: FAIL.

**Step 3: Implement**

In `anticheat/mitigate/metrics.go` add:

```go
var reconcileOutcomes = expvar.NewMap("better_pm_ac_reconcile_outcomes")
var corrections      = expvar.NewInt("better_pm_ac_corrections_total")

// RecordReconcile bumps the per-outcome counter ("accept"/"pending"/"snap").
func RecordReconcile(outcome string) {
    reconcileOutcomes.Add(outcome, 1)
}

// RecordCorrection bumps the total CorrectPlayerMovePrediction send count.
func RecordCorrection() {
    corrections.Add(1)
}
```

Convert the existing aggregate counters to a `*expvar.Map` keyed by `type/subtype` if not already. Update `Apply` to call `violationsByCheck.Add(d.Type()+"/"+d.SubType(), 1)`.

**Step 4: Verify pass**

Run: `go test ./anticheat/mitigate/ -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/mitigate/metrics.go anticheat/mitigate/metrics_reconcile_test.go
git commit -m "[γ.2.7] mitigate: per-check + per-reconcile-outcome expvar maps

Stops aggregating violations into one counter so dashboards can sort by
which check is firing. New maps:
  better_pm_ac_reconcile_outcomes (accept/pending/snap)
  better_pm_ac_corrections_total
And violationsByCheck keyed by Type/SubType."
```

---

### Task T2.8: upstream PlayerAuthInput rewrite

**Files:**
- Modify: `proxy/proxy.go` (PlayerAuthInput case)
- Test: `proxy/upstream_rewrite_test.go`

**Step 1: Failing test**

```go
// proxy/upstream_rewrite_test.go
package proxy

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestUpstreamPlayerAuthInputRewrittenWithCommitted(t *testing.T) {
    p := newTestProxy(t)
    p.Config.Anticheat.Authority.DualTick = true
    p.Config.Anticheat.Authority.UpstreamRewrite = true

    sess := p.AddTestSession(uuid.New())

    // Player committed 0,64,0 last tick. Client now claims 5,64,0 (cheaty);
    // sim Expected is 0.1,64,0; reconcile snaps to expected.
    pl := p.Anticheat.GetPlayer(sess.PlayerID)
    pl.SetExpectedPos(mgl32.Vec3{0.1, 64, 0})
    pl.Commit(mgl32.Vec3{0.1, 64, 0})

    // Simulate upstream forward.
    sent := p.ForwardUpstreamPlayerAuthInputForTest(sess, mgl32.Vec3{5, 64, 0})
    if sent.Position != (mgl32.Vec3{0.1, 64, 0}) {
        t.Fatalf("upstream pos=%v want CommittedPos %v", sent.Position, mgl32.Vec3{0.1, 64, 0})
    }
}
```

**Step 2: Verify fail**

Run: `go test ./proxy/ -run TestUpstream -v`
Expected: FAIL.

**Step 3: Implement**

In `proxy/proxy.go` PlayerAuthInput handling block (around line 149), wrap the upstream forward:

```go
        case *packet.PlayerAuthInput:
            // ... existing OnInput call ...
            if p.cfg.Anticheat.Authority.UpstreamRewrite {
                if pl := p.Anticheat.GetPlayer(sess.PlayerID); pl != nil {
                    pkt.Position = pl.CommittedPos()
                    // Delta should reflect Committed - PrevCommitted to keep
                    // PMMP's velocity tracker sane.
                    pkt.Delta = pl.CommittedPos().Sub(pl.PrevCommittedPos())
                }
            }
            // proceed to upstream forward as before
```

`ForwardUpstreamPlayerAuthInputForTest` is a test seam that exposes the rewrite path — implement as a thin wrapper that calls into the same logic with a captured packet for assertion.

**Step 4: Verify pass**

Run: `go test ./proxy/ -run TestUpstream -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add proxy/proxy.go proxy/upstream_rewrite_test.go
git commit -m "[γ.2.8] proxy: rewrite upstream PlayerAuthInput with CommittedPos

Closes the loop with PMMP: PMMP only sees the proxy-validated position,
so PMMP plugins (including any in-process AC) see consistent state and
can't double-flag. Gated on Authority.UpstreamRewrite."
```

---

### Task T2.9: Scenario integration — legit cases under reconcile flag

**Files:**
- Modify: `anticheat/integration_test.go` (extend legit scenarios)
- Add: 5 new legit scenarios (idle / walk / sprint / jump / swim)

**Step 1: Add failing scenarios**

Add to integration_test.go a parameterized helper that takes config (with Authority.DualTick=true, ReconcileSnap=true) and feeds a 60-tick legit trace, asserting `det[anyCheck].Violations == 0`.

```go
func TestLegitScenariosUnderAuthorityFlag_Walk(t *testing.T) {
    cfg := config.Default().Anticheat
    cfg.Authority.DualTick = true
    cfg.Authority.ReconcileSnap = true

    runLegitTrace(t, cfg, walkTrace())
}
```

`walkTrace` returns 60 ticks of `{tick, pos, onGround, yaw, pitch}` for a player walking forward at vanilla speed (~0.215 b/tick). `runLegitTrace` feeds them through `m.OnInput` and asserts no detection has Violations > 0.

**Step 2: Verify fail (initially expected)**

Run: `go test ./anticheat/ -run Legit -v`
Expected: at least one of the new scenarios may FAIL because reconcile under the γ.1 sim stub computes naive ExpectedPos and snaps. **This is the gate-failure case** — fix in T2.10 by feeding sim with input forces.

**Step 3: Tighten sim stub or add input plumbing**

If walk scenario fails: sim.Step needs `InputForward`/`Sprint` etc. to predict the right ExpectedPos. Plumb them from OnInput (we have inputData bitset; map known Bedrock InputData bits to forward/strafe/sprint/sneak/jump per gophertunnel `protocol.InputFlag*` constants).

Add a small helper `sim.InputsFromBitset(bs InputDataLoader, yaw float32) (forward, strafe float32, sprint, sneak, jump bool)` and use it at the OnInput sim build site.

**Step 4: Verify pass**

Run: `go test ./anticheat/ -run Legit -v`
Expected: PASS for idle/walk/sprint/jump/swim.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/integration_test.go anticheat/sim/walk.go
git commit -m "[γ.2.9] integration: 5 legit traces under DualTick+ReconcileSnap

Idle/walk/sprint/jump/swim. Forced sim.Step to consume Bedrock InputData
bits so the legit walk doesn't snap-back. Phase γ.2 gate: legit scenarios
FP=0 with both flags on. (Cheat scenarios still on β path; γ.3 migrates.)"
```

---

### Task T2.10: γ.2 phase gate

Same shape as T1.10: `go test ./...`, `go vet ./...`, `golangci-lint run ./...`, bench, tag `γ.2-reconcile`, work-board update. Document the legit-scenario FP-rate baseline (target: 0).

---

## Phase γ.3 — Movement Check Migration (W3–5)

목표: Movement 6종(Speed/A, Fly/A, NoFall/A, Phase/A, NoSlow/A, Velocity/A)을 `CommittedPos` + `TickContext` 소비로 재작성, 체크 내부 pos 추적 상태 제거. `Authority.MovementAuth` 플래그 canary → 100%.

각 movement check마다 패턴이 동일하므로 T3.1로 reference task를 자세히, T3.2~T3.6은 동일 형식으로 짧게 기록.

---

### Task T3.1: Speed/A → CommittedPos + AuthorityAuthoritativeMovement (reference task)

**Files:**
- Modify: `anticheat/checks/movement/speed.go`
- Modify: `anticheat/checks/movement/speed_test.go` (existing)
- Test: `anticheat/checks/movement/speed_authority_test.go` (new)

**Step 1: Failing test**

```go
// anticheat/checks/movement/speed_authority_test.go
package movement

import (
    "testing"

    "github.com/boredape874/Better-pm-AC/anticheat/data"
    "github.com/boredape874/Better-pm-AC/anticheat/meta"
    "github.com/boredape874/Better-pm-AC/config"
    "github.com/go-gl/mathgl/mgl32"
    "github.com/google/uuid"
)

func TestSpeedAReadsCommittedDelta(t *testing.T) {
    p := data.NewPlayer(uuid.New(), "u")
    p.Commit(mgl32.Vec3{0, 64, 0})
    p.Commit(mgl32.Vec3{0.5, 64, 0}) // Δ=0.5 b/tick — over default 0.4 cap

    chk := NewSpeedCheck(config.SpeedConfig{Enabled: true, MaxSpeed: 0.4})
    flagged, _ := chk.Check(p, meta.TickContext{ServerTick: 1, ClientTick: 1})
    if !flagged {
        t.Fatalf("Speed/A must flag Δ=0.5 against cap 0.4")
    }
}

func TestSpeedAAuthorityIsMovement(t *testing.T) {
    chk := NewSpeedCheck(config.SpeedConfig{Enabled: true})
    if chk.Authority() != meta.AuthorityAuthoritativeMovement {
        t.Fatalf("Speed/A.Authority=%v want AuthMovement", chk.Authority())
    }
}
```

**Step 2: Verify fail**

Run: `go test ./anticheat/checks/movement/ -run TestSpeedA -v`
Expected: FAIL — `Check` signature mismatch, `Authority` undefined.

**Step 3: Implement**

In `anticheat/checks/movement/speed.go`:

1. Change `Check(p *data.Player) (bool, string)` → `Check(p *data.Player, ctx meta.TickContext) (bool, string)`.
2. Replace internal pos delta calculation with `delta := p.CommittedPos().Sub(p.PrevCommittedPos())`.
3. Use horizontal length: `hLen := mgl32.Vec2{delta.X(), delta.Z()}.Len()`.
4. Add method `func (*SpeedCheck) Authority() meta.Authority { return meta.AuthorityAuthoritativeMovement }`.
5. Remove any `lastPos`, `lastTick` fields from SpeedCheck — they were the old single-source. Now Player owns it.

Update existing `speed_test.go` callers to pass `meta.TickContext{}` and seed `Commit`/`PrevCommittedPos`.

Update Manager `OnInput` Speed/A call site:

```go
        if flagged, info := m.speed.Check(p, ctx); flagged { ... }
```

(Only on the path where `cfg.Authority.MovementAuth` is true; otherwise legacy path. Use a small wrapper:

```go
if m.cfg.Authority.MovementAuth {
    flagged, info := m.speed.Check(p, ctx)
    ...
} else {
    // β legacy path retained until γ.3 ends
    flagged, info := m.speed.CheckLegacy(p)
    ...
}
```

`CheckLegacy` is the old logic copied into a new method, kept for the canary period and deleted in T3.10.)

**Step 4: Verify pass**

Run: `go test ./anticheat/checks/movement/ -run TestSpeedA -v`
Expected: PASS.

Run: `go test ./...`
Expected: all PASS.

**Step 5: Commit**

```bash
git add anticheat/checks/movement/speed.go anticheat/checks/movement/speed_test.go anticheat/checks/movement/speed_authority_test.go anticheat/anticheat.go
git commit -m "[γ.3.1] Speed/A: Committed-delta + AuthorityAuthoritativeMovement

Reads CommittedPos so the check sees post-reconcile position only.
CheckLegacy preserves β behavior under MovementAuth=false until T3.10
removes the dual path."
```

---

### Tasks T3.2–T3.6: Fly/A, NoFall/A, Phase/A, NoSlow/A, Velocity/A

Each follows T3.1 exactly:

1. Add failing test asserting (a) Check consumes CommittedPos / PrevCommittedPos / TickContext, (b) Authority() returns AuthorityAuthoritativeMovement.
2. Migrate Check signature, drop internal pos state.
3. Add CheckLegacy preserving β behavior.
4. Wire Manager dispatch behind `Authority.MovementAuth` flag.
5. Run full test suite.
6. Commit `[γ.3.N] <Check>: Committed-delta migration`.

| Task | Check | Special note |
|---|---|---|
| T3.2 | Fly/A | Reads vertical Δ from CommittedPos diff. GravViolTicks moves into Player. |
| T3.3 | NoFall/A | Damage-event side runs unchanged; sim.Step truth replaces ad-hoc fall accumulator. Add Player.FallDistance() backed by CommittedPos history. |
| T3.4 | Phase/A | Old "is claimed pos in block" → "is committed pos in block" (which is impossible after reconcile snap, so check becomes a *signal* of reconcile snap rate against Phase-shaped traces). |
| T3.5 | NoSlow/A | Item-use bitset → sim input forward force → sim ExpectedPos → CommittedPos delta. |
| T3.6 | Velocity/A | Reads ack queue resolve outcome instead of its own knockback snapshot. KnockbackSnapshot logic moves into ack.Push at the SetActorMotion site. |

Each task: 5 steps + a commit. Allocate 1–2 commits per check.

---

### Task T3.7: Speed/B + Fly/B + NoFall/B migration

The /B variants share patterns with their /A peers and depend on the same Player accessors. One commit per /B check, identical structure to T3.1. 3 commits total.

---

### Task T3.8: Scaffold/A migration

Scaffold/A consumes block-place events, not OnInput pos. But it currently builds its own click→jump correlation; γ.3 just adds `TickContext` to its `Check` signature and reads CommittedPos for the player's Y to decide if "place under feet while jumping" pattern fires. Single commit.

---

### Task T3.9: Manager OnMove path retired or migrated

`OnMove` (legacy MovePlayer fallback) currently has its own duplicated check loop. Either:

- (a) Migrate it to `Authority.MovementAuth` parity by feeding through the same sim+reconcile pipeline, or
- (b) Mark it deprecated — only legacy clients (< MCBE 1.16) ever send MovePlayer instead of PlayerAuthInput, and we don't support those.

Pick (b). Add TODO and a runtime warn-once log if OnMove is hit. Single commit.

---

### Task T3.10: Remove CheckLegacy methods after canary

Once `Authority.MovementAuth=true` is the default and integration suite is green, delete every `CheckLegacy` method introduced in T3.1–T3.6 and remove the `if m.cfg.Authority.MovementAuth` branches in Manager. Single commit.

```bash
git commit -m "[γ.3.10] checks/movement: drop CheckLegacy after canary

Authority.MovementAuth becomes effectively always-on for movement
checks. Flag stays in config for emergency rollback to a vintage
build but new code paths use the post-reconcile CommittedPos only."
```

---

### Task T3.11: Cheat-scenario integration extension

Add 5 new cheat traces (Speed micro 0.42 b/tick, Fly hover, NoFall lava-immune, Phase block-seam, NoSlow eat-sprint). Each must flag at least one violation under `MovementAuth=true`. Two commits (scenarios + assertions).

---

### Task T3.12: γ.3 phase gate

`go test ./...`, vet, lint, bench (regression < 20%), work-board, tag `γ.3-movement-auth`.

---

## Phase γ.4 — Multi-Raycast Combat (W5–6)

목표: combat 헬퍼 + Reach/A + KillAura/A·B 재작성, entity rewind 확장.

---

### Task T4.1: Entity rewind extension — yaw/pitch/bbox

**Files:**
- Modify: `anticheat/entity/buffer.go` (struct + Push + Lookup signature)
- Test: `anticheat/entity/yaw_pitch_test.go`

**Step 1: Failing test**

```go
package entity

import (
    "testing"

    "github.com/go-gl/mathgl/mgl32"
)

func TestSnapshotIncludesYawPitchBBox(t *testing.T) {
    b := NewBuffer(80)
    s := Snapshot{
        Tick:   100,
        Pos:    mgl32.Vec3{0, 64, 0},
        Yaw:    45,
        Pitch:  -10,
        BBoxHalfWidth:  0.3,
        BBoxHeight:     1.8,
    }
    b.Push(s)

    got, ok := b.At(100)
    if !ok || got.Yaw != 45 || got.Pitch != -10 {
        t.Fatalf("At(100)=%+v ok=%v", got, ok)
    }
}
```

**Step 2: Verify fail.**

**Step 3: Extend `Snapshot` struct** with `Yaw, Pitch float32; BBoxHalfWidth, BBoxHeight float32`. Update `Push` callers in Manager and proxy where rewind entries are emitted.

**Step 4: Verify pass.**

**Step 5: Commit `[γ.4.1] entity: rewind snapshots carry yaw/pitch/bbox`.**

---

### Task T4.2: combat.RayCaster scaffold

**Files:**
- Create: `anticheat/combat/raycaster.go`
- Test: `anticheat/combat/raycaster_test.go`

```go
// raycaster.go
package combat

import (
    "github.com/boredape874/Better-pm-AC/anticheat/entity"
    "github.com/go-gl/mathgl/mgl32"
)

type HitResult struct {
    Hit        bool
    NearestT   float32
    SampleTick uint64
}

type RayCaster struct {
    Tracker *entity.Tracker
}

// CastN samples target bbox/pos at clientTick ± window and returns the closest hit.
func (rc *RayCaster) CastN(
    eyeFrom mgl32.Vec3,
    forward mgl32.Vec3,
    targetRID uint64,
    clientTick uint64,
    window uint64,
    maxReach float32,
) HitResult {
    var best HitResult
    best.NearestT = maxReach + 1
    for off := -int(window); off <= int(window); off++ {
        sample := uint64(int(clientTick) + off)
        snap, ok := rc.Tracker.At(targetRID, sample)
        if !ok {
            continue
        }
        t, hit := rayBBoxIntersect(eyeFrom, forward, snap.Pos, snap.BBoxHalfWidth, snap.BBoxHeight, maxReach)
        if hit && t < best.NearestT {
            best = HitResult{Hit: true, NearestT: t, SampleTick: sample}
        }
    }
    return best
}

// rayBBoxIntersect: slab method; returns (t, true) if ray hits within maxReach.
func rayBBoxIntersect(from, fwd, center mgl32.Vec3, hw, h, maxT float32) (float32, bool) {
    minB := center.Sub(mgl32.Vec3{hw, 0, hw})
    maxB := center.Add(mgl32.Vec3{hw, h, hw})
    tmin, tmax := float32(0), maxT
    for axis := 0; axis < 3; axis++ {
        dir := fwd[axis]
        org := from[axis]
        if dir == 0 {
            if org < minB[axis] || org > maxB[axis] {
                return 0, false
            }
            continue
        }
        t1 := (minB[axis] - org) / dir
        t2 := (maxB[axis] - org) / dir
        if t1 > t2 { t1, t2 = t2, t1 }
        if t1 > tmin { tmin = t1 }
        if t2 < tmax { tmax = t2 }
        if tmin > tmax { return 0, false }
    }
    return tmin, true
}
```

Tests cover: hit on direct ray, miss off-axis, multiple ticks pick nearest, beyond reach returns false.

Commit `[γ.4.2] combat: RayCaster.CastN with slab-method bbox intersect`.

---

### Task T4.3: Reach/A on RayCaster

Replace single-point Reach distance with `RayCaster.CastN(eye, forward, targetRID, ctx.ClientTick, 2, cfg.MaxReach)`. Flag when `Hit=false` AND a swing was registered, OR `NearestT > cfg.MaxReach`. Commit `[γ.4.3] Reach/A: multi-raycast on rewound entity`.

---

### Task T4.4: KillAura/A on RayCaster

KillAura/A (swing-less hit) reframed: if `Hit=true` at sample tick T, was there a swing in `[T-1, T+1]`? If not → flag. Commit `[γ.4.4] KillAura/A: swing-window cross-check on hit sample tick`.

---

### Task T4.5: KillAura/B on RayCaster

KillAura/B (FOV) reframed: at sample tick of nearest hit, check `angleBetween(forward, eye→targetCenter) ≤ 90°`. If outside → flag. Commit `[γ.4.5] KillAura/B: FOV check at hit sample tick`.

---

### Task T4.6: Entity bbox/yaw/pitch population from server packets

In proxy/Manager, every `MovePlayer`/`MoveActorAbsolute`/`AddActor` push a `Snapshot` with the new fields. AddActor carries bbox via entity-flags or actor-data; if not directly available, use vanilla defaults (player 0.3 × 1.8) and improve in γ.5. Commit `[γ.4.6] proxy: feed yaw/pitch/bbox into entity rewind`.

---

### Task T4.7: Combat scenario integration

Add 3 new cheat traces (reach 6 b, killaura no-swing, killaura behind-back). All must flag under `Authority.CombatMultiRay=true`. Commit `[γ.4.7] integration: 3 multi-raycast cheat traces`.

---

### Task T4.8: γ.4 phase gate

`go test ./...`, vet, lint, bench (`BenchmarkMultiRaycast` new — must be < 20 µs/cast at 80-tick window), tag `γ.4-multiray-combat`.

---

## Phase γ.5 — Moat Hardening + Replay Harness (W6–7)

---

### Task T5.1: lava-specific drag in sim.fluid

Lava drag = 0.5 (water = 0.8). Add `FluidLava` enum to `sim/fluid.go`, branch in `applyFluid`. Test: vertical fall in lava reaches terminal velocity below water's. Commit.

---

### Task T5.2: bubble column upward drag

Bubble column from soul sand → +0.04 b/tick upward; magma block → -0.06 downward. Block IDs in `world/blocks.go`. Test + commit.

---

### Task T5.3: small-Δ collision precision tier

In `sim/collision.go`, when |Δ| component < 1e-4, use `math.Float32bits` comparison instead of `<` to detect "stuck inside block by epsilon" cases. Test: player position 1e-5 inside a wall must be detected as collision. Commit.

---

### Task T5.4: Block bbox completeness pass — trapdoors

Extend `world/blocks.go` to handle trapdoor 4 directions × open/closed = 8 bboxes. Test fixture per direction. Commit.

---

### Task T5.5: Block bbox completeness pass — fences/walls

Connection state from neighbor query → 16 bboxes per fence/wall. Single helper `connectivityBBox(neighbors)`. Commit.

---

### Task T5.6: Block bbox completeness pass — stairs

7 stair shapes (straight, inner/outer corners × left/right). Helper `stairBBox(facing, shape)`. Commit.

---

### Task T5.7: replay harness recorder

`cmd/replay/recorder.go` — wraps proxy with a flag `--record path.bpac` that writes every client/server packet as `{ts_ns, dir, len, pkt}`. Test on a 100-tick canned session. Commit.

---

### Task T5.8: replay harness player

`cmd/replay/player.go` — reads file, instantiates `anticheat.Manager`, replays packets, outputs final per-check violation map. Diff against golden fixture. Commit.

---

### Task T5.9: 5 golden fixtures

Record 5 traces (idle, walk, sprint+jump, combat, fall) on a local PMMP 5.x dev server. Commit fixtures + golden expected violation maps. Add CI step that runs the player against each fixture.

---

### Task T5.10: γ.5 phase gate

Tag `γ.5-moat`. Update ops-runbook.md with reconcile-snap and correction alerts. Bench, vet, lint.

---

## Phase γ.6 — Deferred γ Items (W7–8)

---

### Task T6.1: world block-update history

Extend `world/tracker.go` with a per-position ring buffer of `{tick, prevID, newID}` for the last 200 ticks. API: `History(pos) []BlockEvent`. Commit.

---

### Task T6.2: Nuker/A — multi-break rate

`anticheat/checks/world/nuker_a.go`: flag when `>= cfg.MaxBreaksPerSec` distinct-position breaks within 1 s. Test + commit.

---

### Task T6.3: FastBreak/A — break-time below mining-time

Compare actual break interval against vanilla mining-time table for the block + tool. Below 80% → flag. Commit.

---

### Task T6.4: FastPlace/A — finalize from γ stub

Promote the existing β-WIP `fastplace_a.go` to fully tested + registered in Manager. Commit.

---

### Task T6.5: Tower/A — jump-place chain

Detect ≥ 4 consecutive jump-and-place-below cycles within 2 s. Commit.

---

### Task T6.6: InvalidBreak/A — break through wall

Flag when broken-block position has no LOS from player eye (raycast through world). Commit.

---

### Task T6.7: Manager.OnLogin + LoginData struct

`anticheat/data/login.go`: `LoginData{Protocol int, ClientVersion string, DeviceOS uint32, DeviceModel string, GameVersion string, ...}`. Manager.OnLogin captures from `Login` packet. Commit.

---

### Task T6.8: Protocol/A

Flag when `LoginData.Protocol` doesn't match any known MCBE protocol version in our table. Commit.

---

### Task T6.9: EditionFaker/A

Cross-check `LoginData.GameVersion` (advertised) vs `LoginData.Protocol` (actual). Mismatch → flag. Commit.

---

### Task T6.10: ClientSpoof/A

Flag known cheat-client signatures (`DeviceModel` patterns, missing/abnormal `ClientRandomID`). Commit.

---

### Task T6.11: γ.6 phase gate + v1.0-rc tag

`go test ./...`, full bench, replay harness CI green, v1.0-rc release notes draft. Tag `v1.0-rc1`.

---

## Final v1.0 release tasks (after γ.6 stable in canary)

- **R1:** v1.0 release notes consolidation (CHANGELOG.md).
- **R2:** Publish design doc + plan as a paired blog post (Korean + English).
- **R3:** Update README.md to reflect v1.0 capability matrix (replace "γ 진행 중" lines).
- **R4:** Tag `v1.0` and push.

---

## References

- Design: `docs/plans/2026-04-21-oomph-parity-plus-design.md` (commit 1a2af17).
- β baseline: `docs/plans/2026-04-19-anticheat-overhaul-design.md`.
- Roadmap board: `docs/plans/2026-04-19-work-board.md`.
- Spec docs: `docs/check-specs/`.
- Ops runbook: `docs/ops-runbook.md`.
