package app

import (
	"strings"

	"github.com/dnrtk/popup_translator/internal/config"
	"github.com/dnrtk/popup_translator/internal/prompt"
)

// PopupService is bound to the popup frontend.
type PopupService struct {
	app *App
}

// NewPopupService returns the frontend binding for a.
func NewPopupService(a *App) *PopupService { return &PopupService{app: a} }

// Languages returns the target languages for the dropdown.
func (s *PopupService) Languages() []prompt.Language {
	return prompt.Languages
}

// State returns the current popup content (used when the page (re)loads).
func (s *PopupService) State() PopupState {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	return s.app.state
}

// Retranslate translates text again into target. It is called when the
// user changes the target language or edits the source text.
func (s *PopupService) Retranslate(text, target string) {
	if !prompt.IsSupported(target) || strings.TrimSpace(text) == "" {
		return
	}
	if s.app.Config().TargetMode == config.TargetModeRemember {
		s.app.updateConfig(func(c *config.Config) { c.LastTarget = target })
	}
	s.app.translate(text, target, false)
}

// Copy puts text on the clipboard.
func (s *PopupService) Copy(text string) bool {
	return s.app.wapp.Clipboard.SetText(text)
}

// Hide closes the popup.
func (s *PopupService) Hide() {
	s.app.HidePopup()
}

// ServerState returns the llama-server state.
func (s *PopupService) ServerState() ServerStateEvent {
	return s.app.ServerState()
}
