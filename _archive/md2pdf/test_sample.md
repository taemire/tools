# TKCLI 사용자 매뉴얼

TACHYON CLI(TKCLI)는 TACHYON 시스템을 효율적으로 관리하기 위한 통합 명령줄 도구입니다.

## 1. 설치 및 설정

TKCLI는 단일 바이너리로 제공되어 별도의 설치 과정 없이 바로 사용 가능합니다.

### 1.1 시스템 요구사항

- 운영체제: Windows, Linux, macOS
- 메모리: 최소 512MB
- 디스크: 50MB 이상

### 1.2 다운로드 및 설치

1. 공식 배포 페이지에서 바이너리 다운로드
2. 다운로드한 파일을 원하는 위치에 저장
3. PATH 환경변수에 추가

## 2. 기본 사용법

TKCLI 명령어는 다음과 같은 형식으로 사용합니다:

```bash
tkcli [command] [options]
```

### 2.1 도움말 보기

```bash
tkcli --help
tkcli [command] --help
```

## 3. 주요 명령어

### 3.1 서비스 관리

- `status` - 서비스 상태 확인
- `start` - 서비스 시작
- `stop` - 서비스 중지
- `restart` - 서비스 재시작

### 3.2 설정 관리

> **참고**: 설정 변경 후에는 서비스를 재시작해야 합니다.

설정 파일의 위치:
- Windows: `C:\ProgramData\nProtect\config.yml`
- Linux: `/etc/nprotect/config.yml`

## 4. 고급 기능

TKCLI는 고급 사용자를 위한 추가 기능을 제공합니다.

### 4.1 배치 처리

여러 명령을 순차적으로 실행할 수 있습니다.

### 4.2 로그 분석

실시간 로그 모니터링 및 분석 기능을 제공합니다.
