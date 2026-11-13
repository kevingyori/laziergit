package repo

import (
	"os"

	"github.com/jesseduffield/lazygit/pkg/common"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/i18n"
	"github.com/jesseduffield/lazygit/pkg/logs"
	"github.com/spf13/afero"
)

// Placeholder is a repository that isn't open yet: the user's global lazygit
// config (from the current environment) and nothing else. It lets a window
// lay out its panels and skeletons before git has answered anything; it
// must never run git (its Git is nil).
func Placeholder() (*Repo, error) {
	tempDir, err := os.MkdirTemp("", "laziergit-*")
	if err != nil {
		return nil, err
	}
	appConfig, err := config.NewAppConfig("lazygit", "laziergit", "", "", "laziergit", false, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}
	userConfig := appConfig.GetUserConfig()
	cmn := &common.Common{
		Log:      logs.NewProductionLogger(),
		Tr:       i18n.EnglishTranslationSet(),
		AppState: appConfig.GetAppState(),
		Fs:       afero.NewOsFs(),
	}
	cmn.SetUserConfig(userConfig)
	return &Repo{AppConfig: appConfig, Common: cmn, Config: userConfig, tempDir: tempDir}, nil
}

// IsPlaceholder reports whether r is a Placeholder.
func (r *Repo) IsPlaceholder() bool { return r.Git == nil }
