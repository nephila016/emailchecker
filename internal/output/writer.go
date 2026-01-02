package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yourusername/emailverify/internal/verifier"
)

// Writer interface for different output formats
type Writer interface {
	Write(result *verifier.Result) error
	Flush() error
	Close() error
}

// Format represents output format type
type Format string

const (
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
	FormatJSONL Format = "jsonl"
	FormatTXT   Format = "txt"
)

// DetectFormat detects output format from filename
func DetectFormat(filename string) Format {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		return FormatJSON
	case ".csv":
		return FormatCSV
	case ".jsonl", ".ndjson":
		return FormatJSONL
	default:
		return FormatTXT
	}
}

// NewWriter creates a writer for the given format and file
func NewWriter(filename string, format Format) (Writer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	switch format {
	case FormatJSON:
		return NewJSONWriter(file), nil
	case FormatCSV:
		return NewCSVWriter(file), nil
	case FormatJSONL:
		return NewJSONLWriter(file), nil
	default:
		return NewTXTWriter(file), nil
	}
}

// JSONWriter writes results as JSON array
type JSONWriter struct {
	file    *os.File
	results []*verifier.Result
	mu      sync.Mutex
}

func NewJSONWriter(file *os.File) *JSONWriter {
	return &JSONWriter{
		file:    file,
		results: make([]*verifier.Result, 0),
	}
}

func (w *JSONWriter) Write(result *verifier.Result) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.results = append(w.results, result)
	return nil
}

func (w *JSONWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.file.Seek(0, 0)
	w.file.Truncate(0)

	encoder := json.NewEncoder(w.file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(w.results)
}

func (w *JSONWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// JSONLWriter writes results as JSON Lines (one JSON per line)
type JSONLWriter struct {
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
}

func NewJSONLWriter(file *os.File) *JSONLWriter {
	return &JSONLWriter{
		file:    file,
		encoder: json.NewEncoder(file),
	}
}

func (w *JSONLWriter) Write(result *verifier.Result) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(result)
}

func (w *JSONLWriter) Flush() error {
	return w.file.Sync()
}

func (w *JSONLWriter) Close() error {
	return w.file.Close()
}

// CSVWriter writes results as CSV
type CSVWriter struct {
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
	header bool
}

func NewCSVWriter(file *os.File) *CSVWriter {
	w := &CSVWriter{
		file:   file,
		writer: csv.NewWriter(file),
	}
	// Write header
	w.writer.Write([]string{
		"email",
		"valid",
		"status",
		"status_code",
		"reason",
		"disposable",
		"role_account",
		"free_provider",
		"catch_all",
		"mx_host",
		"confidence_score",
		"latency_ms",
		"verified_at",
	})
	w.header = true
	return w
}

func (w *CSVWriter) Write(result *verifier.Result) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.writer.Write([]string{
		result.Email,
		fmt.Sprintf("%t", result.Valid),
		string(result.Status),
		fmt.Sprintf("%d", result.StatusCode),
		result.Reason,
		fmt.Sprintf("%t", result.Disposable),
		fmt.Sprintf("%t", result.RoleAccount),
		fmt.Sprintf("%t", result.FreeProvider),
		fmt.Sprintf("%t", result.CatchAll),
		result.MXHost,
		fmt.Sprintf("%d", result.ConfidenceScore),
		fmt.Sprintf("%d", result.LatencyMs),
		result.VerifiedAt.Format("2006-01-02 15:04:05"),
	})
}

func (w *CSVWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writer.Flush()
	return w.writer.Error()
}

func (w *CSVWriter) Close() error {
	w.Flush()
	return w.file.Close()
}

// TXTWriter writes valid emails as plain text (one per line)
type TXTWriter struct {
	file *os.File
	mu   sync.Mutex
}

func NewTXTWriter(file *os.File) *TXTWriter {
	return &TXTWriter{file: file}
}

func (w *TXTWriter) Write(result *verifier.Result) error {
	if !result.Valid && result.Status != verifier.StatusRisky {
		return nil // Only write valid/risky emails
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := fmt.Fprintf(w.file, "%s\n", result.Email)
	return err
}

func (w *TXTWriter) Flush() error {
	return w.file.Sync()
}

func (w *TXTWriter) Close() error {
	return w.file.Close()
}

// MultiWriter writes to multiple outputs
type MultiWriter struct {
	writers []Writer
}

func NewMultiWriter(writers ...Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (w *MultiWriter) Write(result *verifier.Result) error {
	for _, writer := range w.writers {
		if err := writer.Write(result); err != nil {
			return err
		}
	}
	return nil
}

func (w *MultiWriter) Flush() error {
	for _, writer := range w.writers {
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (w *MultiWriter) Close() error {
	for _, writer := range w.writers {
		if err := writer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// WriteResultsToFile writes all results to a file
func WriteResultsToFile(filename string, results []*verifier.Result) error {
	format := DetectFormat(filename)
	writer, err := NewWriter(filename, format)
	if err != nil {
		return err
	}
	defer writer.Close()

	for _, result := range results {
		if err := writer.Write(result); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// StreamWriter for console output
type StreamWriter struct {
	writer io.Writer
	mu     sync.Mutex
}

func NewStreamWriter(w io.Writer) *StreamWriter {
	return &StreamWriter{writer: w}
}

func (w *StreamWriter) Write(result *verifier.Result) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := fmt.Fprintf(w.writer, "%s: %s\n", result.Email, result.Status)
	return err
}

func (w *StreamWriter) Flush() error {
	return nil
}

func (w *StreamWriter) Close() error {
	return nil
}
