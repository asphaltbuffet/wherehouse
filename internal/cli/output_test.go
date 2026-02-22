package cli

import (
	"bytes"
	"testing"

	"github.com/goccy/go-json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOutputWriter(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}

	w := NewOutputWriter(out, err, false, false)

	require.NotNil(t, w)
	assert.False(t, w.jsonMode)
	assert.False(t, w.quietMode)
	assert.NotNil(t, w.styles)
}

func TestOutputWriter_Success_Normal(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Success("Success message")

	output := out.String()
	assert.Contains(t, output, "Success message")
	assert.Empty(t, err.String())
}

func TestOutputWriter_Success_QuietMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, true)

	w.Success("Success message")

	assert.Empty(t, out.String())
	assert.Empty(t, err.String())
}

func TestOutputWriter_Success_JSONMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, true, false)

	w.Success("Success message")

	var data map[string]string
	require.NoError(t, json.Unmarshal(out.Bytes(), &data))
	assert.Equal(t, "success", data["status"])
	assert.Equal(t, "Success message", data["message"])
}

func TestOutputWriter_Success_JSONAndQuiet(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, true, true)

	w.Success("Success message")

	// Quiet mode takes precedence - suppresses output
	assert.Empty(t, out.String())
}

func TestOutputWriter_Error_Normal(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Error("Error message")

	output := err.String()
	assert.Contains(t, output, "Error")
	assert.Contains(t, output, "Error message")
	assert.Empty(t, out.String())
}

func TestOutputWriter_Error_AlwaysShown(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, true)

	w.Error("Error message")

	// Error ignores quiet mode
	assert.Contains(t, err.String(), "Error message")
}

func TestOutputWriter_Error_JSONMode(t *testing.T) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	w := NewOutputWriter(out, errBuf, true, false)

	w.Error("Error message")

	// JSON output goes to stdout (out), not stderr
	var data map[string]string
	require.NoError(t, json.Unmarshal(out.Bytes(), &data))
	assert.Equal(t, "error", data["status"])
	assert.Equal(t, "Error message", data["message"])
}

func TestOutputWriter_Warning_Normal(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Warning("Warning message")

	output := err.String()
	assert.Contains(t, output, "Warning")
	assert.Contains(t, output, "Warning message")
	assert.Empty(t, out.String())
}

func TestOutputWriter_Warning_QuietMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, true)

	w.Warning("Warning message")

	assert.Empty(t, out.String())
	assert.Empty(t, err.String())
}

func TestOutputWriter_Warning_JSONMode(t *testing.T) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	w := NewOutputWriter(out, errBuf, true, false)

	w.Warning("Warning message")

	// JSON output goes to stdout (out), not stderr
	var data map[string]string
	require.NoError(t, json.Unmarshal(out.Bytes(), &data))
	assert.Equal(t, "warning", data["status"])
	assert.Equal(t, "Warning message", data["message"])
}

func TestOutputWriter_Info_Normal(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Info("Info message")

	output := out.String()
	assert.Contains(t, output, "Info message")
	assert.Empty(t, err.String())
}

func TestOutputWriter_Info_QuietMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, true)

	w.Info("Info message")

	assert.Empty(t, out.String())
	assert.Empty(t, err.String())
}

func TestOutputWriter_Info_JSONMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, true, false)

	w.Info("Info message")

	// Info not printed in JSON mode
	assert.Empty(t, out.String())
}

func TestOutputWriter_KeyValue_Normal(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.KeyValue("key", "value")

	output := out.String()
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")
	assert.Empty(t, err.String())
}

func TestOutputWriter_KeyValue_QuietMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, true)

	w.KeyValue("key", "value")

	assert.Empty(t, out.String())
	assert.Empty(t, err.String())
}

func TestOutputWriter_KeyValue_JSONMode(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, true, false)

	w.KeyValue("key", "value")

	var data map[string]string
	require.NoError(t, json.Unmarshal(out.Bytes(), &data))
	assert.Equal(t, "value", data["key"])
}

func TestOutputWriter_JSON_ValidData(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	data := map[string]any{
		"name":  "test",
		"value": 42,
	}

	require.NoError(t, w.JSON(data))

	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, "test", result["name"])
	assert.InEpsilon(t, 42, result["value"], 0.0001)
}

func TestOutputWriter_JSON_ComplexData(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	data := map[string]any{
		"nested": map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		"array": []string{"a", "b", "c"},
	}

	require.NoError(t, w.JSON(data))

	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.NotNil(t, result["nested"])
	assert.NotNil(t, result["array"])
}

func TestOutputWriter_Print_Unformatted(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Print("plain text")

	assert.Equal(t, "plain text", out.String())
}

func TestOutputWriter_Print_BypassesQuiet(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, true)

	w.Print("plain text")

	// Print bypasses quiet mode
	assert.Equal(t, "plain text", out.String())
}

func TestOutputWriter_Println_WithNewline(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Println("line 1")
	w.Println("line 2")

	output := out.String()
	assert.Contains(t, output, "line 1\n")
	assert.Contains(t, output, "line 2\n")
}

func TestOutputWriter_EmptyMessages(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	require.NotNil(t, w)

	// These should not panic
	require.NotPanics(t, func() { w.Success("") })
	require.NotPanics(t, func() { w.Error("") })
	require.NotPanics(t, func() { w.Warning("") })
	require.NotPanics(t, func() { w.Info("") })
	require.NotPanics(t, func() { w.KeyValue("", "") })
	require.NotPanics(t, func() { w.Print("") })
	require.NotPanics(t, func() { w.Println("") })
}

func TestOutputWriter_MultipleOperations(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	w.Info("Starting...")
	w.KeyValue("Config", "/path/to/config")
	w.Success("Operation completed")

	output := out.String()
	assert.Contains(t, output, "Starting...")
	assert.Contains(t, output, "Config")
	assert.Contains(t, output, "Operation completed")
}

func TestOutputWriter_StylesNotNil(t *testing.T) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	w := NewOutputWriter(out, err, false, false)

	require.NotNil(t, w.styles)
	assert.NotNil(t, w.styles.Success)
	assert.NotNil(t, w.styles.Error)
	assert.NotNil(t, w.styles.Warning)
	assert.NotNil(t, w.styles.Info)
	assert.NotNil(t, w.styles.Key)
	assert.NotNil(t, w.styles.Value)
}
