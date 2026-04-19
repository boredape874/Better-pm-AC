# Agent Protocol — 다중 AI 협업 규칙

> **문서 위치**: `docs/plans/2026-04-19-agent-protocol.md`
> **역할**: 이 저장소에서 일하는 모든 AI 에이전트가 따라야 하는 **작업 규칙**과 **경계선**을 정의.
> **업데이트 권한**: Orchestrator(AI-O) 만. 다른 AI는 WORK-BOARD에 `Proposal` 항목으로 제안.

---

## 0. 읽기 순서 (TL;DR)

이 저장소에 들어온 AI가 **첫 메시지에서 반드시 해야 할 것**:

1. `docs/plans/2026-04-19-anticheat-overhaul-design.md` 전체 읽기 (§0~§14).
2. 이 문서(`AGENT-PROTOCOL.md`) 전체 읽기.
3. `docs/plans/2026-04-19-work-board.md` 열기 → 본인 역할 섹션에서 `pending` Task 찾기.
4. Task가 있으면 **Claim 절차(§3)** 수행.
5. 작업 진행.
6. 완료 시 **Complete 절차(§4)** 수행.

**읽기 단계를 건너뛰고 코드를 바꾸지 마세요.** 파일 경계를 모르면 충돌이 발생하고, 설계 결정을 모르면 헛수고가 됩니다.

---

## 1. AI 역할 정의

이 프로젝트에는 **7가지 역할**이 있습니다. 하나의 AI 에이전트 세션은 **하나의 역할**만 수행합니다. 사용자가 역할을 지정하며(예: "AI-S로 작업해"), 지정되지 않았으면 **Orchestrator(AI-O)로 간주**합니다.

| 역할 | 이름 | 담당 패키지 | 주요 Task |
|------|------|------------|----------|
| **AI-O** | Orchestrator | `anticheat/meta/`, `anticheat/anticheat.go`, `proxy/`, `config/`, `main.go`, `docs/` | 인터페이스 관리, Phase 1·5a·5b 전체, 통합, Proposal 승인 |
| **AI-W** | World | `anticheat/world/**` | 청크 파서, 블록 lookup, BBox |
| **AI-S** | Simulation | `anticheat/sim/**` | 물리 엔진 (β + γ) |
| **AI-E** | Entity | `anticheat/entity/**` | Rewind 링버퍼 |
| **AI-A** | Ack | `anticheat/ack/**` | NetworkStackLatency 마커 |
| **AI-M** | Mitigate | `anticheat/mitigate/**` | Correction policy 실행 |
| **AI-C** | Check | `anticheat/checks/**`, `docs/check-specs/` | 44개 체크 구현/개조 |

**AI-C는 추가로 서브 분화 가능**: AI-C-1 (movement), AI-C-2 (combat), AI-C-3 (packet), AI-C-4 (world checks), AI-C-5 (client checks). 병렬 실행 시 WORK-BOARD에서 어느 서브카테고리를 담당할지 명시.

---

## 2. 파일 경계 규칙 (충돌 방지)

### 2.1 소유권 테이블

| 경로 | 소유자 | 비고 |
|------|--------|------|
| `anticheat/meta/**` | **AI-O** | 인터페이스 · Detection · 공통 타입 |
| `anticheat/anticheat.go` | **AI-O** | Manager 배선 |
| `anticheat/data/player.go` | **AI-O** | 스키마 변경 필요 시. 다른 AI는 읽기만. 필드 추가는 Proposal 경유. |
| `anticheat/world/**` | AI-W | 전권 |
| `anticheat/sim/**` | AI-S | 전권 |
| `anticheat/entity/**` | AI-E | 전권 |
| `anticheat/ack/**` | AI-A | 전권 |
| `anticheat/mitigate/**` | AI-M | 전권 |
| `anticheat/checks/**` | AI-C | 전권 (서브카테고리 단위) |
| `proxy/**` | **AI-O** | MiTM 레이어 |
| `config/**` | **AI-O** | 스키마 |
| `main.go`, `go.mod`, `go.sum` | **AI-O** | 의존성 · 엔트리 |
| `docs/check-specs/**` | AI-C | 체크 스펙 문서 |
| `docs/plans/**` | **AI-O** | 설계 문서 |

### 2.2 절대 규칙

1. **자기 소유가 아닌 파일은 수정 금지.** 버그 발견 시 `Proposal`로 제안.
2. **같은 파일을 2개 AI가 동시에 수정 금지.** 이는 `git merge conflict`의 주 원인.
3. **인터페이스 파일(`anticheat/meta/interfaces.go`)은 AI-O 단독 수정.** 다른 AI는 "이 필드가 필요하다"고 Proposal 제출.
4. **공유 유틸(`anticheat/data/player.go` 등)에 필드 추가 시**: AI-O가 먼저 추가 → 해당 AI가 사용. 동시 수정 금지.

### 2.3 예외 처리

- 긴급 빌드 깨짐 수정은 **AI-O만 가능**. 다른 AI는 `pending` → `blocked`로 표시하고 대기.
- README.md, LICENSE, .gitignore는 AI-O만.

---

## 3. Task Claim 절차

### 3.1 WORK-BOARD에서 Task 찾기

`docs/plans/2026-04-19-work-board.md` 열고 본인 역할의 섹션에서 **`Status: pending`** 이고 **dependency가 충족된** Task를 선택.

### 3.2 Claim 커밋

Task 항목의 `Status` 필드를 **`claimed-by-<역할>-<타임스탬프>`** 로 변경 후 **단일 커밋**:

```
git add docs/plans/2026-04-19-work-board.md
git commit -m "[board] claim 2.5 sim engine core"
```

커밋 메시지 형식: `[board] claim <task-id> <short-desc>`.

이 커밋은 **작업 시작 선언**입니다. 다른 AI는 이후 같은 Task를 claim하지 않아야 합니다.

### 3.3 Dependency 확인

WORK-BOARD의 Task 항목에는 `Depends on:` 필드가 있습니다. 해당 Task들이 `Status: done`이 아니면 **절대 착수 금지**. 기다리거나 다른 Task를 claim.

### 3.4 동시 claim 경합 해소

드물지만 두 AI가 거의 동시에 같은 Task를 claim 커밋하는 경우:
- `git pull --rebase` 후 conflict 발생 시 **타임스탬프가 더 빠른 쪽이 승**.
- 진 AI는 자기 claim 커밋을 `git reset --hard HEAD~1`로 취소하고 다른 Task 찾기.
- **orchestrator에게 보고 불필요** — 이 규칙으로 자동 해소.

---

## 4. 작업 진행 · 완료 절차

### 4.1 진행 중 상태

Task를 claim하고 코드 작성 중이면 `Status: in-progress`. 여러 날 걸리는 큰 Task는 중간 progress를 `Notes:` 필드에 기록:

```
### Task 2.5 — sim engine core Step()
Status: in-progress
Claimed: ai-s 2026-04-20T09:00Z
Notes:
  2026-04-21: walk/jump/gravity 구현 완료, collision 착수
  2026-04-22: collision 4방향 BBox sweep 90% 완성
```

### 4.2 Blocker

불가능하거나 다른 Task 대기 필요 시:

```
Status: blocked
Blocker: Task 1.3 (interface stub)이 아직 done이 아님. 인터페이스가 확정되어야 구현 시작 가능.
```

`blocked` 상태는 주기적으로 다시 확인. Orchestrator가 `AGENT-PROTOCOL.md` 위반한 경우 개입.

### 4.3 완료 커밋

구현 + 빌드 통과 + 해당 Task의 `Acceptance:` 기준 충족 시:

1. 코드 커밋들은 각각 `[phase.task] <verb> <subject>` 형식:
   ```
   [2.5] add sim.Engine.Step() core
   [2.5] implement gravity + air drag
   [2.5] add friction/surface logic
   [2.5] tests for β physics invariants
   ```

2. 마지막에 WORK-BOARD 업데이트:
   ```
   Status: done
   Completed: ai-s 2026-04-25T14:30Z
   PR: <링크 또는 "merged to main">
   ```
   커밋: `[board] complete 2.5`.

3. Follow-up Task가 있으면 WORK-BOARD에 새로 추가 (자기 권한 내).

### 4.4 빌드 녹색 보장

**완료 선언 전 반드시**:
```bash
go build ./...
go vet ./...
go test ./anticheat/<내패키지>/...
```

**빨강이면 `done` 금지.** 수정 불가능한 이유(외부 의존)면 `blocked`.

---

## 5. 커밋 메시지 컨벤션

### 5.1 형식

```
[<phase>.<task>] <imperative-verb> <short-subject>

<optional body>

Co-Authored-By: <AI-name> <noreply@anthropic.com>
```

- **phase.task**: WORK-BOARD의 Task ID (`2.5`, `3.12` 등).
- **board 커밋**은 `[board] claim|complete|reassign <task-id>` 형식.
- **verb**: `add / implement / fix / refactor / test / doc / remove`. 현재형·명령형.
- **subject**: 소문자, 마침표 없음, 70자 이내.

### 5.2 예시

```
[2.5] add sim.Engine.Step() core loop

Walks through applyInput → gravity → friction → collision in the
order Bedrock source uses. Surface flags updated after collision.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
```

```
[board] claim 3.12 migrate Fly/A to sim-based diff
```

```
[3.12] migrate Fly/A to sim-diff comparison

Replace hover-ticks heuristic with Y-delta = |client.y − sim.y|.
Tolerance scales with latency. Keeps TrustDuration semantics.
```

### 5.3 금지

- `--no-verify` 금지 (훅 실패는 원인 해결).
- `--amend` 금지 (새 커밋 만들기).
- 한 커밋에 여러 Task 섞기 금지.
- "Fix stuff", "WIP" 같은 메시지 금지.

---

## 6. 인터페이스 변경 절차 (AI-O 전용)

### 6.1 Proposal 제출 (요청자: 누구든)

WORK-BOARD의 최상단 `## Proposals` 섹션에 항목 추가:

```
### Proposal P-00X — <간단 제목>
Requested by: AI-S
Date: 2026-04-22
Context: sim.Engine.Step()에서 Y 좌표 보정에 현재 tick의 knockback velocity가 필요.
현재 SimInput에는 knockback 필드 없음. 추가 필요.
Proposed change:
  SimInput 구조체에 `KnockbackApplied mgl32.Vec3` 필드 추가.
  설계 §11.2 업데이트.
Affected files:
  - anticheat/meta/interfaces.go
  - anticheat/sim/engine.go (자기 구현)
  - design.md §11.2
Status: pending-review
```

### 6.2 AI-O 검토

AI-O는 주기적으로 Proposals 확인:
- **수락**: 설계 문서 + 인터페이스 파일 수정, Proposal `Status: accepted, implemented in commit <sha>`.
- **거부**: `Status: rejected, reason: <설명>`.
- **토론 필요**: `Status: pending-discussion` + 사용자에게 보고.

### 6.3 긴급 인터페이스 변경

빌드를 깨트리는 인터페이스 불일치 발견 시:
1. 발견 AI는 **즉시** 자기 Task를 `blocked`.
2. WORK-BOARD `## Urgent` 섹션에 기록.
3. AI-O가 우선순위로 해결.

---

## 7. 충돌 발생 시 에스컬레이션

### 7.1 Git merge conflict

- **같은 파일 동시 수정 금지 원칙이 지켜졌다면 conflict는 일어날 수 없음.**
- 발생했다면 규칙 위반. `git log --all` 으로 누가 경계 침범했는지 확인 후 WORK-BOARD `## Urgent`에 기록, AI-O 개입.
- **자의적 conflict resolution 금지** (특히 다른 AI의 변경 삭제).

### 7.2 빌드 깨짐

- 자기 Task로 인한 것: 즉시 수정 또는 커밋 revert.
- 타 AI 원인: WORK-BOARD `## Urgent`에 경위 기록. 자기 Task는 `blocked`.
- AI-O가 root cause 분석 후 담당 AI에게 전달.

### 7.3 설계 해석 불일치

같은 설계 문단을 두 AI가 다르게 해석 → 서로 다른 구현:
- 먼저 완성한 AI의 해석이 기본 우선.
- 반대 해석이 정당하다면 WORK-BOARD Proposals에서 재논의.
- AI-O 판정.

### 7.4 성능 목표 미달

`go test -bench` 결과가 §1.4 목표 미달 시:
- 미달 원인이 특정 패키지 → 그 AI에게 최적화 Task 발급.
- 구조적 원인(설계 한계) → Proposal로 설계 수정 제안.

---

## 8. 테스트 규칙

### 8.1 단위 테스트

각 AI는 자기 패키지의 **exported API 전부**에 대해 최소 1개 테스트 작성.

### 8.2 통합 테스트

`anticheat/integration_test.go`는 **AI-O만 작성**. 개별 AI는 자기 컴포넌트의 stub/mock만 제공.

### 8.3 골든 로그 (sim 검증)

AI-S는 물리 시뮬 검증을 위해 `anticheat/sim/testdata/*.log` 에 실측 Bedrock 세션 녹화 비교. 녹화 생성 툴은 Phase 2에서 AI-S가 작성.

### 8.4 CI (향후)

현재는 로컬 `go build ./... && go vet ./... && go test ./...` 만 요구. GitHub Actions CI 추가는 **Phase 5b Task 5b.1** (β 릴리스 직전).

---

## 9. 코드 리뷰 / PR

### 9.1 현재 방식 (간소)

- 각 Task는 **단일 PR** 또는 main에 직접 push (작은 Task).
- 큰 Task는 PR 생성: `gh pr create --title "[2.5] sim engine core" --body "<WORK-BOARD 링크>"`.
- PR 승인자: **AI-O** (인터페이스·배선 영향) 또는 **사용자**.

### 9.2 리뷰 기준

- 설계 §11 인터페이스 위반 → 차단.
- 파일 경계 위반 → 차단.
- 빌드/테스트 빨강 → 차단.
- 코드 스타일(§10) 위반 → 수정 요청.
- "동작하지만 설계와 다름" → Proposal 요구.

---

## 10. Git Worktree 사용 (선택적 권장)

### 10.1 언제 쓰나

- 병렬 AI가 5개 이상일 때.
- 긴 Task(3일 이상)로 main에 WIP을 올리기 싫을 때.

### 10.2 생성

```bash
# AI-S가 자기 worktree 생성
git worktree add ../Better-pm-AC-sim feat/sim-core
cd ../Better-pm-AC-sim
# 작업...
git push origin feat/sim-core
gh pr create
```

### 10.3 정리

PR 머지 후:
```bash
git worktree remove ../Better-pm-AC-sim
git branch -d feat/sim-core
```

---

## 11. 보안 · 민감 데이터

- 커밋에 토큰·비밀번호 금지.
- `config.toml`의 실제 서버 주소는 예시로만.
- 플레이어 UUID 로그는 개발 디버그 외 남기지 말 것 (GDPR 고려).

---

## 12. 문서 갱신 권한

| 문서 | 수정 가능자 |
|------|------------|
| `design.md` | AI-O만 (Proposal 수락 후) |
| `AGENT-PROTOCOL.md` | AI-O만 |
| `WORK-BOARD.md` | 누구나 (자기 Task 섹션만, Claim/Complete 규칙 준수) |
| `docs/check-specs/*.md` | AI-C (자기 체크 스펙) |
| `docs/forks-notes.md` | AI-O |
| `README.md` | AI-O |

---

## 13. 사용자 지시 우선순위

- **사용자(champ) 지시 > 이 프로토콜 > 설계 문서 > 기존 코드 패턴**.
- 사용자가 이 프로토콜과 다른 지시를 내리면 그 지시를 따르되, **그 지시를 AI-O가 이 프로토콜에 반영** (영구화).
- 사용자가 "AI-S야, X 해줘"라고 하면 다른 세션의 AI-S도 동일한 지시 반영.

---

## 14. 시작 체크리스트 (새 AI 첫 메시지)

복사해서 쓰세요:

```
체크리스트:
[ ] design.md 읽음 (§0-§14 전체)
[ ] AGENT-PROTOCOL.md 읽음 (이 문서)
[ ] WORK-BOARD.md 열어 본인 역할(AI-?) 섹션 확인
[ ] 현재 활성 Proposal 목록 확인
[ ] pending Task 중 dependency 충족된 것 선택
[ ] claim 커밋 작성 준비
```

5분 안에 전부 체크 가능. 못 채웠으면 사용자에게 확인 요청.

---

**문서 끝.** 실제 Task 목록은 [WORK-BOARD.md](./2026-04-19-work-board.md).
