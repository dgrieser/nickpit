package modelcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
)

type CachedCapabilities struct {
	BaseURL    string                   `json:"base_url"`
	DetectedAt time.Time                `json:"detected_at"`
	Capability config.ModelCapabilities `json:"capability"`
}

type capabilityCacheFile struct {
	Entries []CachedCapabilities `json:"entries"`
}

func DefaultCachePath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("NICKPIT_CACHE_DIR")); dir != "" {
		return filepath.Join(dir, "model-capabilities.json"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("modelcheck cache: resolving user cache dir: %w", err)
	}
	return filepath.Join(dir, "nickpit", "model-capabilities.json"), nil
}

func NormalizeBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func NormalizeModel(model string) string {
	return strings.TrimSpace(model)
}

func FindProfileCapability(profile config.Profile) (config.ModelCapabilities, bool) {
	model := NormalizeModel(profile.Model)
	for _, capability := range profile.SupportedModels {
		if NormalizeModel(capability.Model) == model {
			return cloneCapability(capability), true
		}
	}
	return config.ModelCapabilities{}, false
}

func ReadCachedCapability(path, baseURL, model string) (config.ModelCapabilities, bool, error) {
	file, err := readCacheFile(path)
	if err != nil {
		return config.ModelCapabilities{}, false, err
	}
	baseURL = NormalizeBaseURL(baseURL)
	model = NormalizeModel(model)
	for _, entry := range file.Entries {
		if NormalizeBaseURL(entry.BaseURL) == baseURL && NormalizeModel(entry.Capability.Model) == model {
			return cloneCapability(entry.Capability), true, nil
		}
	}
	return config.ModelCapabilities{}, false, nil
}

func WriteCachedCapability(path, baseURL string, capability config.ModelCapabilities, detectedAt time.Time) error {
	if path == "" {
		return fmt.Errorf("modelcheck cache: empty path")
	}
	file, err := readCacheFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	baseURL = NormalizeBaseURL(baseURL)
	capability = cloneCapability(capability)
	capability.Model = NormalizeModel(capability.Model)
	entry := CachedCapabilities{
		BaseURL:    baseURL,
		DetectedAt: detectedAt.UTC(),
		Capability: capability,
	}
	replaced := false
	for i, existing := range file.Entries {
		if NormalizeBaseURL(existing.BaseURL) == baseURL && NormalizeModel(existing.Capability.Model) == capability.Model {
			file.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		file.Entries = append(file.Entries, entry)
	}
	return writeCacheFile(path, file)
}

func CapabilityFromResult(result Result) config.ModelCapabilities {
	summary := result.Summary()
	return config.ModelCapabilities{
		Model:        result.Model,
		Compatible:   summary.Compatible,
		Response:     summary.Response,
		Reasoning:    config.ReasoningCapabilities(summary.Reasoning),
		Tools:        summary.Tools,
		JSONSchema:   cloneBoolPtr(summary.JSONSchema),
		JSONResponse: cloneBoolPtr(summary.JSONResponse),
	}
}

func ResultFromCapability(capability config.ModelCapabilities, useJSONSchema bool) Result {
	effort := firstEffort(capability.Reasoning.Efforts)
	probes := []ProbeResult{
		{Name: "configured_no_tools", ReasoningEffort: effort, Reasoned: capability.Reasoning.Traces, Status: statusFor(capability.Response)},
		{Name: "configured_tools", ReasoningEffort: effort, Tools: true, Status: statusFor(capability.Tools)},
	}
	probes = append(probes,
		ProbeResult{Name: "configured_json_output", ReasoningEffort: effort, Status: optionalStatus(capability.JSONResponse), Error: optionalError(capability.JSONResponse)},
		ProbeResult{Name: "configured_json_schema", ReasoningEffort: effort, Status: optionalStatus(capability.JSONSchema), Error: optionalError(capability.JSONSchema)},
	)
	return Result{
		Model:            capability.Model,
		ConfiguredEffort: effort,
		UseJSONSchema:    useJSONSchema,
		Probes:           probes,
		PassedEfforts:    append([]string(nil), capability.Reasoning.Efforts...),
	}
}

func readCacheFile(path string) (capabilityCacheFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return capabilityCacheFile{}, os.ErrNotExist
		}
		return capabilityCacheFile{}, fmt.Errorf("modelcheck cache: reading %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return capabilityCacheFile{}, nil
	}
	var file capabilityCacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return capabilityCacheFile{}, fmt.Errorf("modelcheck cache: parsing %s: %w", path, err)
	}
	return file, nil
}

func writeCacheFile(path string, file capabilityCacheFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("modelcheck cache: creating directory: %w", err)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("modelcheck cache: encoding: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".model-capabilities-*.tmp")
	if err != nil {
		return fmt.Errorf("modelcheck cache: creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("modelcheck cache: writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("modelcheck cache: closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("modelcheck cache: replacing %s: %w", path, err)
	}
	return nil
}

func cloneCapability(capability config.ModelCapabilities) config.ModelCapabilities {
	capability.Reasoning.Efforts = append([]string(nil), capability.Reasoning.Efforts...)
	capability.JSONSchema = cloneBoolPtr(capability.JSONSchema)
	capability.JSONResponse = cloneBoolPtr(capability.JSONResponse)
	return capability
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func statusFor(ok bool) Status {
	if ok {
		return StatusOK
	}
	return StatusFailed
}

func optionalStatus(ok *bool) Status {
	if ok == nil {
		return StatusFailed
	}
	return statusFor(*ok)
}

func optionalError(ok *bool) string {
	if ok == nil {
		return "capability not stored"
	}
	return ""
}

func firstEffort(efforts []string) string {
	if len(efforts) == 0 {
		return ""
	}
	return efforts[0]
}
