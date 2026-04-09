# Better-pm-AC

고성능 Minecraft Bedrock Edition 안티치트 프록시 — Go + [Gophertunnel](https://github.com/Sandertv/gophertunnel) 기반의 MiTM(Man-in-the-Middle) 방식으로 동작합니다.

```
플레이어 → [Better-pm-AC 프록시] → (TCP/RakNet) → [PMMP 서버]
```

---

## 특징

| 기능 | 설명 |
|------|------|
| **MiTM 프록시** | 클라이언트와 PMMP 서버 사이에서 모든 패킷을 투명하게 전달 |
| **Speed** | 수평 이동 속도가 설정값 초과 시 감지 |
| **Fly** | 공중에서 Y축 속도가 0에 가까울 때(호버링) 감지 |
| **NoFall** | 낙하 후 착지 시 낙하 거리가 vanilla 기준치(3블록)를 초과할 경우 감지 |
| **Reach** | 공격 대상이 설정된 최대 도달 거리 초과 시 감지 |
| **KillAura** | vanilla 최대 CPS(~16)를 초과하는 초고속 연속 공격 감지 |
| **자동 킥** | 위반 횟수가 임계값 도달 시 자동으로 클라이언트 연결 종료 |
| **TOML 설정** | 모든 검사 항목의 활성화 여부 및 임계값을 `config.toml`에서 조정 가능 |

---

## 동작 방식

```
클라이언트 (Bedrock)
        │
        ▼ RakNet / UDP
┌───────────────────┐
│  Better-pm-AC     │
│  (Go MiTM Proxy)  │  ← 패킷 가로채기 + 안티치트 검사
└───────────────────┘
        │
        ▼ RakNet / UDP
  PMMP 서버 (PHP)
```

모든 클라이언트 패킷은 Better-pm-AC에서 먼저 수신 → 안티치트 로직 실행 → 이상 없으면 PMMP로 전달됩니다.  
서버에서 클라이언트 방향 패킷은 수정 없이 투명하게 전달됩니다.

---

## 프로젝트 구조

```
Better-pm-AC/
├── main.go                          # 진입점
├── config/
│   └── config.go                    # TOML 설정 로드/저장
├── proxy/
│   ├── proxy.go                     # MiTM 프록시 핵심 로직
│   └── session.go                   # 플레이어 세션 쌍
├── anticheat/
│   ├── anticheat.go                 # 매니저 (검사 조율 + 킥)
│   ├── data/
│   │   └── player.go                # 플레이어별 상태 추적
│   └── checks/
│       ├── movement/
│       │   ├── speed.go             # Speed 검사
│       │   ├── fly.go               # Fly 검사
│       │   └── nofall.go            # NoFall 검사
│       └── combat/
│           ├── reach.go             # Reach 검사
│           └── killaura.go          # KillAura 검사
└── go.mod / go.sum
```

---

## 빠른 시작

### 요구사항

- Go 1.24 이상
- PMMP 서버 (PocketMine-MP 5.x 권장)

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

처음 실행 시 기본값으로 `config.toml`이 자동 생성됩니다.

### 포트 설정

| 구분 | 기본값 |
|------|--------|
| 프록시 리슨 | `0.0.0.0:19132` |
| PMMP 서버 | `127.0.0.1:19133` |

PMMP의 리슨 포트를 `19133`으로 변경하거나, `config.toml`의 `remote_addr` 값을 수정하세요.

---

## 설정 (`config.toml`)

```toml
[proxy]
listen_addr = "0.0.0.0:19132"
remote_addr = "127.0.0.1:19133"

[anticheat.speed]
enabled    = true
max_speed  = 0.7    # blocks/tick (20tps 기준)
violations = 10

[anticheat.fly]
enabled    = true
violations = 5

[anticheat.nofall]
enabled    = true
violations = 5

[anticheat.reach]
enabled    = true
max_reach  = 3.1    # blocks
violations = 5

[anticheat.killaura]
enabled    = true
violations = 5
```

---

## 참고 프로젝트

- [Gophertunnel](https://github.com/Sandertv/gophertunnel) — Bedrock 프로토콜 구현체
- [Oomph](https://github.com/oomph-ac/oomph) — Go 기반 Bedrock 안티치트 프록시 (참고)
- [Spectrum-PM](https://github.com/spectrum-pm/spectrum) — PMMP 플러그인 방식 안티치트 (참고)

---

## 라이선스

[MIT](LICENSE)
