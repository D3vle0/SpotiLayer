package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"SpotiLayer/internal/config"
	"SpotiLayer/internal/server"
	"SpotiLayer/internal/spotify"
)

type UI struct {
	app           fyne.App
	window        fyne.Window
	cfg           *config.Config
	spotifyClient *spotify.Client
	server        *server.Server

	clientIDEntry     *widget.Entry
	clientSecretEntry *widget.Entry
	portEntry         *widget.Entry
	redirectURILabel  *widget.Label

	statusBadge     *widget.Label
	serverToggleBtn *widget.Button
	loginBtn        *widget.Button
	copyOBSBtn      *widget.Button
	saveBtn         *widget.Button

	nowPlayingLabel *widget.Label
	loginStatusText *widget.Label
}

func NewUI(cfg *config.Config, spClient *spotify.Client, srv *server.Server, fontBytes []byte) *UI {
	a := app.NewWithID("com.spotilayer.app")
	if len(fontBytes) > 0 {
		a.Settings().SetTheme(newCustomKoreanTheme(fontBytes))
	}

	w := a.NewWindow("SpotiLayer")
	w.Resize(fyne.NewSize(450, 420))
	w.CenterOnScreen()

	ui := &UI{
		app:           a,
		window:        w,
		cfg:           cfg,
		spotifyClient: spClient,
		server:        srv,
	}

	ui.setupComponents()
	ui.setupEvents()
	ui.buildLayout()

	// Connect server status listener
	srv.SetOnStatusChange(func(running bool, addr string) {
		ui.updateServerStatusUI(running, addr)
	})

	return ui
}

func (ui *UI) setupComponents() {
	ui.clientIDEntry = widget.NewEntry()
	ui.clientIDEntry.SetPlaceHolder("Spotify Client ID 입력")
	ui.clientIDEntry.SetText(ui.cfg.ClientID)

	ui.clientSecretEntry = widget.NewPasswordEntry()
	ui.clientSecretEntry.SetPlaceHolder("Spotify Client Secret 입력")
	ui.clientSecretEntry.SetText(ui.cfg.ClientSecret)

	ui.portEntry = widget.NewEntry()
	ui.portEntry.SetPlaceHolder("8888")
	ui.portEntry.SetText(strconv.Itoa(ui.cfg.Port))

	initialPort := ui.cfg.Port
	if initialPort <= 0 {
		initialPort = 8888
	}
	ui.redirectURILabel = widget.NewLabel(fmt.Sprintf("http://127.0.0.1:%d/callback", initialPort))

	ui.statusBadge = widget.NewLabel("🔴 서버 중지됨")
	ui.statusBadge.TextStyle = fyne.TextStyle{Bold: true}

	ui.loginStatusText = widget.NewLabel("로그인: 미인증")
	ui.nowPlayingLabel = widget.NewLabel("🎵 재생 중인 음악 없음")
	ui.nowPlayingLabel.Wrapping = fyne.TextWrapWord

	ui.saveBtn = widget.NewButton("💾 설정 저장", nil)

	ui.serverToggleBtn = widget.NewButton("▶ 서버 시작", nil)
	ui.serverToggleBtn.Importance = widget.HighImportance

	ui.loginBtn = widget.NewButton("🌐 Spotify 로그인", nil)
	ui.loginBtn.Disable()

	ui.copyOBSBtn = widget.NewButton("📋 OBS URL 복사", nil)
	ui.copyOBSBtn.Disable()
}

func (ui *UI) setupEvents() {
	// Port change updates redirect URI label
	ui.portEntry.OnChanged = func(s string) {
		port, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || port <= 0 {
			port = 8888
		}
		ui.redirectURILabel.SetText(fmt.Sprintf("http://127.0.0.1:%d/callback", port))
	}

	// Save Settings button
	ui.saveBtn.OnTapped = func() {
		ui.saveInputsToConfig()
		if err := ui.cfg.Save(); err != nil {
			dialog.ShowError(fmt.Errorf("설정 저장 실패: %w", err), ui.window)
		} else {
			dialog.ShowInformation("저장 완료", "config.json에 설정이 저장되었습니다.", ui.window)
		}
	}

	// Server start / stop toggle
	ui.serverToggleBtn.OnTapped = func() {
		if ui.server.IsRunning() {
			if err := ui.server.Stop(); err != nil {
				dialog.ShowError(fmt.Errorf("서버 중지 실패: %w", err), ui.window)
			}
		} else {
			ui.saveInputsToConfig()
			if ui.cfg.ClientID == "" || ui.cfg.ClientSecret == "" {
				dialog.ShowInformation("안내", "Spotify Client ID와 Client Secret을 먼저 입력해주세요.", ui.window)
				return
			}

			ui.spotifyClient.UpdateCredentials(ui.cfg.ClientID, ui.cfg.ClientSecret)

			if err := ui.server.Start(ui.cfg.Host, ui.cfg.Port); err != nil {
				dialog.ShowError(fmt.Errorf("서버 시작 실패: %w", err), ui.window)
			}
		}
	}

	// Spotify Login button
	ui.loginBtn.OnTapped = func() {
		addr := ui.server.GetAddress()
		if addr == "" {
			dialog.ShowInformation("안내", "먼저 서버를 시작해주세요.", ui.window)
			return
		}
		parsedURL, err := url.Parse(addr + "/login")
		if err == nil {
			_ = ui.app.OpenURL(parsedURL)
		}
	}

	// Copy OBS Source URL
	ui.copyOBSBtn.OnTapped = func() {
		addr := ui.server.GetAddress()
		if addr == "" {
			port := ui.cfg.Port
			if port <= 0 {
				port = 8888
			}
			addr = fmt.Sprintf("http://127.0.0.1:%d", port)
		}
		ui.window.Clipboard().SetContent(addr)
		dialog.ShowInformation("복사 완료", "OBS 브라우저 소스 URL이 클립보드에 복사되었습니다:\n"+addr, ui.window)
	}
}

func (ui *UI) saveInputsToConfig() {
	ui.cfg.ClientID = strings.TrimSpace(ui.clientIDEntry.Text)
	ui.cfg.ClientSecret = strings.TrimSpace(ui.clientSecretEntry.Text)
	port, err := strconv.Atoi(strings.TrimSpace(ui.portEntry.Text))
	if err == nil && port > 0 {
		ui.cfg.Port = port
	} else {
		ui.cfg.Port = 8888
	}
	ui.spotifyClient.UpdateCredentials(ui.cfg.ClientID, ui.cfg.ClientSecret)
}

func (ui *UI) buildLayout() {
	// Top Header
	titleLabel := widget.NewLabel("🎧 SpotiLayer")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	headerBox := container.NewHBox(titleLabel, layout.NewSpacer(), ui.statusBadge)

	// Redirect URI Row with Copy button
	copyRedirectBtn := widget.NewButton("📋 복사", func() {
		ui.window.Clipboard().SetContent(ui.redirectURILabel.Text)
		dialog.ShowInformation("복사 완료", "Spotify 대시보드 등록용 Redirect URI가 복사되었습니다:\n"+ui.redirectURILabel.Text, ui.window)
	})

	redirectBox := container.NewBorder(nil, nil, nil, copyRedirectBtn, ui.redirectURILabel)

	// Settings Form (플랫한 폼 구성 - Card 컨테이너 박스 미사용)
	form := widget.NewForm(
		widget.NewFormItem("Client ID", ui.clientIDEntry),
		widget.NewFormItem("Client Secret", ui.clientSecretEntry),
		widget.NewFormItem("포트 (Port)", ui.portEntry),
		widget.NewFormItem("Redirect URI", redirectBox),
	)

	// Action Buttons (2열 그리드)
	buttonsGrid := container.NewGridWithColumns(2,
		ui.saveBtn,
		ui.serverToggleBtn,
		ui.loginBtn,
		ui.copyOBSBtn,
	)

	// Bottom Status Rows
	statusBox := container.NewVBox(
		ui.loginStatusText,
		ui.nowPlayingLabel,
	)

	// Flat Main Layout: Card 컨테이너 없이 깔끔한 단일 화면 배치
	mainContent := container.NewVBox(
		headerBox,
		widget.NewSeparator(),
		form,
		widget.NewSeparator(),
		buttonsGrid,
		widget.NewSeparator(),
		statusBox,
	)

	ui.window.SetContent(container.NewPadded(mainContent))
}

func (ui *UI) updateServerStatusUI(running bool, addr string) {
	fyne.Do(func() {
		if running {
			ui.statusBadge.SetText("🟢 서버 실행 중")
			ui.serverToggleBtn.SetText("⏹ 서버 중지")
			ui.loginBtn.Enable()
			ui.copyOBSBtn.Enable()
		} else {
			ui.statusBadge.SetText("🔴 서버 중지됨")
			ui.serverToggleBtn.SetText("▶ 서버 시작")
			ui.loginBtn.Disable()
			ui.copyOBSBtn.Disable()
			ui.nowPlayingLabel.SetText("🎵 재생 중인 음악 없음")
		}
	})
}

func (ui *UI) startStatusPoller() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if !ui.server.IsRunning() {
				continue
			}

			if ui.spotifyClient.IsAuthorized() {
				track, err := ui.spotifyClient.GetCurrentlyPlaying("")
				var playingText string
				if err == nil && track != nil && track.Title != "" {
					playingText = fmt.Sprintf("🎵 %s - %s", track.Title, track.Artist)
				} else {
					playingText = "🎵 재생 중인 음악 없음"
				}

				fyne.Do(func() {
					ui.loginStatusText.SetText("로그인: 인증 완료 ✅")
					ui.nowPlayingLabel.SetText(playingText)
				})
			} else {
				fyne.Do(func() {
					ui.loginStatusText.SetText("로그인: 미인증 (로그인 필요)")
				})
			}
		}
	}()
}

func (ui *UI) Run() {
	ui.startStatusPoller()

	// Auto-start server if configured or credentials are present and valid
	if ui.cfg.ClientID != "" && ui.cfg.ClientSecret != "" {
		ui.spotifyClient.UpdateCredentials(ui.cfg.ClientID, ui.cfg.ClientSecret)
		if ui.cfg.AutoStart {
			_ = ui.server.Start(ui.cfg.Host, ui.cfg.Port)
		}
	}

	ui.window.ShowAndRun()
}
