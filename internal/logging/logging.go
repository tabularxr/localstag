package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	level      Level
	service    string
	traceID    string
	metrics    *Metrics
	mu         sync.RWMutex
}

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

type Metrics struct {
	mu                sync.RWMutex
	EventsProcessed   int64
	ErrorsOccurred    int64
	TotalProcessTime  time.Duration
	LastProcessTime   time.Time
	AverageProcessTime time.Duration
	ThroughputPerSec  float64
	LastCalculated    time.Time
}

type PipelineContext struct {
	TraceID     string
	StagID      string
	AnchorID    string
	EventType   string
	ClientID    string
	SessionID   string
	FrameNumber uint64
	BatchID     string
	Component   string
}

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func (l Level) Icon() string {
	switch l {
	case DEBUG:
		return "🔍"
	case INFO:
		return "ℹ️"
	case WARN:
		return "⚠️"
	case ERROR:
		return "❌"
	default:
		return "❓"
	}
}

func NewLogger(level string, service string) *Logger {
	var l Level
	switch level {
	case "debug":
		l = DEBUG
	case "info":
		l = INFO
	case "warn":
		l = WARN
	case "error":
		l = ERROR
	default:
		l = INFO
	}

	return &Logger{
		level:   l,
		service: service,
		metrics: &Metrics{
			LastCalculated: time.Now(),
		},
	}
}

func (l *Logger) WithContext(ctx *PipelineContext) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	newLogger := &Logger{
		level:   l.level,
		service: l.service,
		traceID: ctx.TraceID,
		metrics: l.metrics,
	}
	
	return newLogger
}

func (l *Logger) StartTimer() func() time.Duration {
	start := time.Now()
	return func() time.Duration {
		duration := time.Since(start)
		l.updateMetrics(duration)
		return duration
	}
}

func (l *Logger) updateMetrics(duration time.Duration) {
	l.metrics.mu.Lock()
	defer l.metrics.mu.Unlock()
	
	l.metrics.EventsProcessed++
	l.metrics.TotalProcessTime += duration
	l.metrics.LastProcessTime = time.Now()
	
	if l.metrics.EventsProcessed > 0 {
		l.metrics.AverageProcessTime = l.metrics.TotalProcessTime / time.Duration(l.metrics.EventsProcessed)
	}
	
	// Calculate throughput every 10 seconds
	if time.Since(l.metrics.LastCalculated) > 10*time.Second {
		l.metrics.ThroughputPerSec = float64(l.metrics.EventsProcessed) / time.Since(l.metrics.LastCalculated).Seconds()
		l.metrics.LastCalculated = time.Now()
	}
}

func (l *Logger) GetMetrics() *Metrics {
	l.metrics.mu.RLock()
	defer l.metrics.mu.RUnlock()
	
	return &Metrics{
		EventsProcessed:    l.metrics.EventsProcessed,
		ErrorsOccurred:     l.metrics.ErrorsOccurred,
		TotalProcessTime:   l.metrics.TotalProcessTime,
		LastProcessTime:    l.metrics.LastProcessTime,
		AverageProcessTime: l.metrics.AverageProcessTime,
		ThroughputPerSec:   l.metrics.ThroughputPerSec,
		LastCalculated:     l.metrics.LastCalculated,
	}
}

func (l *Logger) log(level Level, msg string, ctx *PipelineContext, args ...interface{}) {
	if level < l.level {
		return
	}

	if level == ERROR {
		l.metrics.mu.Lock()
		l.metrics.ErrorsOccurred++
		l.metrics.mu.Unlock()
	}

	timestamp := time.Now().Format("15:04:05.000")
	
	// Build context string
	var contextParts []string
	if ctx != nil {
		if ctx.TraceID != "" {
			contextParts = append(contextParts, fmt.Sprintf("trace=%s", ctx.TraceID[:8]))
		}
		if ctx.StagID != "" {
			contextParts = append(contextParts, fmt.Sprintf("stag=%s", ctx.StagID))
		}
		if ctx.AnchorID != "" {
			contextParts = append(contextParts, fmt.Sprintf("anchor=%s", ctx.AnchorID))
		}
		if ctx.EventType != "" {
			contextParts = append(contextParts, fmt.Sprintf("type=%s", ctx.EventType))
		}
		if ctx.ClientID != "" {
			contextParts = append(contextParts, fmt.Sprintf("client=%s", ctx.ClientID))
		}
		if ctx.FrameNumber > 0 {
			contextParts = append(contextParts, fmt.Sprintf("frame=%d", ctx.FrameNumber))
		}
		if ctx.BatchID != "" {
			contextParts = append(contextParts, fmt.Sprintf("batch=%s", ctx.BatchID))
		}
	}
	
	if l.traceID != "" {
		contextParts = append(contextParts, fmt.Sprintf("trace=%s", l.traceID[:8]))
	}
	
	contextStr := ""
	if len(contextParts) > 0 {
		contextStr = fmt.Sprintf("[%s]", strings.Join(contextParts, " "))
	}
	
	// Format message with key-value pairs
	message := msg
	if len(args) > 0 {
		message += " "
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				message += fmt.Sprintf("%v=%v ", args[i], args[i+1])
			} else {
				message += fmt.Sprintf("%v ", args[i])
			}
		}
	}

	// Color coding for different log levels
	var colorCode string
	switch level {
	case DEBUG:
		colorCode = "\033[36m" // Cyan
	case INFO:
		colorCode = "\033[32m" // Green
	case WARN:
		colorCode = "\033[33m" // Yellow
	case ERROR:
		colorCode = "\033[31m" // Red
	}
	resetCode := "\033[0m"

	// Enhanced log format with service, level icon, and context
	logMessage := fmt.Sprintf("%s%s [%s] %s%s %s %s%s %s%s%s",
		colorCode, level.Icon(), l.service, level.String(), resetCode,
		timestamp,
		colorCode, contextStr, resetCode,
		colorCode, message, resetCode)

	if level == ERROR {
		log.Println(logMessage)
		// Also write to stderr for errors
		fmt.Fprintln(os.Stderr, logMessage)
	} else {
		log.Println(logMessage)
	}
}

// Pipeline-aware logging methods
func (l *Logger) PipelineDebug(ctx *PipelineContext, msg string, args ...interface{}) {
	l.log(DEBUG, msg, ctx, args...)
}

func (l *Logger) PipelineInfo(ctx *PipelineContext, msg string, args ...interface{}) {
	l.log(INFO, msg, ctx, args...)
}

func (l *Logger) PipelineWarn(ctx *PipelineContext, msg string, args ...interface{}) {
	l.log(WARN, msg, ctx, args...)
}

func (l *Logger) PipelineError(ctx *PipelineContext, msg string, args ...interface{}) {
	l.log(ERROR, msg, ctx, args...)
}

// Traditional logging methods (backward compatibility)
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, nil, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, nil, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, msg, nil, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, nil, args...)
}

func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(ERROR, msg, nil, args...)
	os.Exit(1)
}

// High-level pipeline operation logging
func (l *Logger) LogPipelineStart(ctx *PipelineContext) {
	l.PipelineInfo(ctx, "🚀 Pipeline operation started")
}

func (l *Logger) LogPipelineSuccess(ctx *PipelineContext, duration time.Duration) {
	l.PipelineInfo(ctx, "✅ Pipeline operation completed", "duration", duration)
}

func (l *Logger) LogPipelineFailure(ctx *PipelineContext, duration time.Duration, err error) {
	l.PipelineError(ctx, "❌ Pipeline operation failed", "duration", duration, "error", err)
}

func (l *Logger) LogStagHealth(stagID string, healthy bool, anchorCount int, lastActivity time.Time) {
	ctx := &PipelineContext{StagID: stagID}
	if healthy {
		l.PipelineInfo(ctx, "💚 Stag healthy", "anchors", anchorCount, "last_activity", time.Since(lastActivity))
	} else {
		l.PipelineWarn(ctx, "💔 Stag unhealthy", "anchors", anchorCount, "last_activity", time.Since(lastActivity))
	}
}

func (l *Logger) LogPerformanceMetrics() {
	metrics := l.GetMetrics()
	l.Info("📊 Performance metrics",
		"events_processed", metrics.EventsProcessed,
		"errors", metrics.ErrorsOccurred,
		"avg_processing_time", metrics.AverageProcessTime,
		"throughput_per_sec", fmt.Sprintf("%.2f", metrics.ThroughputPerSec),
		"total_processing_time", metrics.TotalProcessTime,
	)
}

// Utility function to generate trace IDs
func GenerateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}