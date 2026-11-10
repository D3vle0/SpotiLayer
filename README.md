# SpotiLayer

## 개요

**SpotiLayer**는 Spotify에서 현재 재생 중인 음악 정보(곡 제목, 아티스트, 앨범 아트)를 실시간으로 표시해주는 방송 오버레이 프로그램입니다.  
OBS Studio, XSplit 등의 방송 녹화 프로그램에서 **'브라우저 소스'**로 등록하여 스트리밍 화면에 예쁜 음악 위젯을 띄울 수 있습니다.

Go 언어 기반의 단일 바이너리 GUI 애플리케이션으로 동작하여 별도의 런타임이나 복잡한 환경 설정 없이 바로 실행할 수 있습니다.

## 사전 준비: Spotify API 키 발급

1. [Spotify for Developers 대시보드](https://developer.spotify.com/dashboard)에 로그인합니다.
2. **Create app**을 클릭합니다.
   - **App name**: 원하는 이름 (예: `SpotiLayer`)
   - **App description**: 원하는 설명
   - **Redirect URIs**: `http://127.0.0.1:8888/callback`  
     *(Spotify 정책상 `localhost` 대신 `127.0.0.1`을 등록해야 합니다. 포트를 변경할 경우 변경한 포트로 등록하며, 프로그램 내 복사 버튼으로 쉽게 복사할 수 있습니다.)*
   - **Which API/SDKs are you planning to use?**: `Web API` 체크
   - 약관 동의 체크 후 **Save**를 누릅니다.
3. 앱 페이지의 **Basic Information**으로 들어가 `Client ID`와 `Client Secret`(View client secret 클릭)을 확인합니다.

## 빌드 및 실행 방법

### 릴리즈 다운로드 (추천)
[Releases](https://github.com/D3vle0/SpotiLayer/releases) 페이지에서 사용 중인 운영체제와 아키텍처에 맞는 최신 바이너리를 다운로드하여 실행할 수 있습니다.

### 수동으로 소스코드 빌드

Go 1.20 이상 환경에서 아래 명령어를 실행합니다.

```bash
# 의존성 설치 및 빌드
make build
# 또는
go build -ldflags="-s -w" -o SpotiLayer .
```

### 실행

생성된 실행 파일을 더블클릭하거나 터미널에서 실행합니다.

```bash
./SpotiLayer
# 또는
make run
```

실행하면 GUI 창이 열립니다.

1. **Client ID**와 **Client Secret**을 입력합니다.
2. **💾 설정 저장** 버튼을 클릭하여 설정을 저장합니다.
3. **▶ 서버 시작** 버튼을 누릅니다.
4. **🌐 Spotify 로그인** 버튼을 눌러 브라우저에서 Spotify 계정을 연동합니다.
5. **📋 OBS URL 복사** 버튼을 눌러 URL(`http://127.0.0.1:8888`)을 복사한 뒤, OBS의 '브라우저 소스'에 등록합니다.

## OBS Studio 연동 방법

1. OBS Studio의 **소스 목록**에서 `+` 버튼을 누르고 **브라우저**를 선택합니다.
2. **URL**에 `http://127.0.0.1:8888` (또는 지정한 포트)을 입력합니다.
3. **너비(Width)**: `420`, **높이(Height)**: `120`을 입력합니다.
4. Spotify에서 음악을 재생하면 방송 화면에 실시간으로 둥근 음악 위젯이 표시됩니다!
