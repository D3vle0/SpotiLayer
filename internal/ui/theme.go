package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type customKoreanTheme struct {
	base fyne.Theme
	font fyne.Resource
}

func newCustomKoreanTheme(fontBytes []byte) fyne.Theme {
	var res fyne.Resource
	if len(fontBytes) > 0 {
		res = fyne.NewStaticResource("AppleSDGothicNeo.ttf", fontBytes)
	}
	return &customKoreanTheme{
		base: theme.DefaultTheme(),
		font: res,
	}
}

func (t *customKoreanTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.base.Color(name, variant)
}

func (t *customKoreanTheme) Font(style fyne.TextStyle) fyne.Resource {
	if t.font != nil {
		return t.font
	}
	return t.base.Font(style)
}

func (t *customKoreanTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *customKoreanTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
