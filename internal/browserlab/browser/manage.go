package browser

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

type Manager struct {
	Store          registry.FileStore
	SessionChecker SessionChecker
	ImageRemover   ImageRemover
}

type DisableRequest struct {
	ImageTag string `json:"imageTag"`
}

type DisableResult struct {
	Record registry.BrowserRecord `json:"record"`
}

type UninstallRequest struct {
	ImageTag           string `json:"imageTag"`
	DeleteImage        bool   `json:"deleteImage"`
	ConfirmDeleteImage bool   `json:"confirmDeleteImage"`
}

type UninstallResult struct {
	Record       registry.BrowserRecord `json:"record"`
	ImageDeleted bool                   `json:"imageDeleted"`
}

type ActiveSession struct {
	ID             string `json:"id"`
	BrowserName    string `json:"browserName"`
	BrowserVersion string `json:"browserVersion"`
	ImageTag       string `json:"imageTag,omitempty"`
}

type SessionChecker interface {
	ActiveSessionsForBrowser(ctx context.Context, record registry.BrowserRecord) ([]ActiveSession, error)
}

type ImageRemover interface {
	RemoveImage(ctx context.Context, imageTag string) error
}

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (problem Problem) Error() string {
	return problem.Message
}

func (manager Manager) Disable(_ context.Context, request DisableRequest) (DisableResult, error) {
	record, err := manager.Store.Disable(request.ImageTag)
	if err != nil {
		return DisableResult{}, Problem{Code: "browser_not_installed", Message: err.Error()}
	}
	return DisableResult{Record: record}, nil
}

func (manager Manager) Uninstall(ctx context.Context, request UninstallRequest) (UninstallResult, error) {
	imageTag := strings.TrimSpace(request.ImageTag)
	if imageTag == "" {
		return UninstallResult{}, Problem{Code: "invalid_uninstall_request", Message: "imageTag is required"}
	}
	if request.DeleteImage && !request.ConfirmDeleteImage {
		return UninstallResult{}, Problem{
			Code:    "image_delete_confirmation_required",
			Message: "deleteImage requires confirmDeleteImage=true because Docker images may be shared",
		}
	}

	record, ok, err := manager.Store.FindByImageTag(imageTag)
	if err != nil {
		return UninstallResult{}, err
	}
	if !ok {
		return UninstallResult{}, Problem{Code: "browser_not_installed", Message: fmt.Sprintf("browser image %q is not installed", imageTag)}
	}

	sessions, err := manager.sessionChecker().ActiveSessionsForBrowser(ctx, record)
	if err != nil {
		return UninstallResult{}, err
	}
	if len(sessions) > 0 {
		return UninstallResult{}, Problem{
			Code:    "active_sessions_block_uninstall",
			Message: fmt.Sprintf("cannot uninstall Chrome %s while %d active session(s) are using it", record.Version, len(sessions)),
		}
	}

	deleted, ok, err := manager.Store.DeleteByImageTag(imageTag)
	if err != nil {
		return UninstallResult{}, err
	}
	if !ok {
		return UninstallResult{}, Problem{Code: "browser_not_installed", Message: fmt.Sprintf("browser image %q is not installed", imageTag)}
	}

	imageDeleted := false
	if request.DeleteImage {
		if err := manager.imageRemover().RemoveImage(ctx, imageTag); err != nil {
			return UninstallResult{}, Problem{Code: "image_delete_failed", Message: err.Error()}
		}
		imageDeleted = true
	}

	return UninstallResult{Record: deleted, ImageDeleted: imageDeleted}, nil
}

func (manager Manager) sessionChecker() SessionChecker {
	if manager.SessionChecker != nil {
		return manager.SessionChecker
	}
	return noActiveSessions{}
}

func (manager Manager) imageRemover() ImageRemover {
	if manager.ImageRemover != nil {
		return manager.ImageRemover
	}
	return DockerImageRemover{}
}

type noActiveSessions struct{}

func (noActiveSessions) ActiveSessionsForBrowser(context.Context, registry.BrowserRecord) ([]ActiveSession, error) {
	return nil, nil
}

type DockerImageRemover struct{}

func (DockerImageRemover) RemoveImage(ctx context.Context, imageTag string) error {
	cmd := exec.CommandContext(ctx, "docker", "image", "rm", imageTag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		} else {
			message = message + ": " + err.Error()
		}
		return fmt.Errorf("delete Docker image %s: %s", imageTag, message)
	}
	return nil
}
