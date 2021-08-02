package main

import (
	"io"
	"time"
)

type StreamLoggerConfig struct {
	Src         io.ReadWriter // Actual IO stream that gets logged
	Dest        io.Writer     // The destination io.Writer
	TimeFormat  string        // Time Format string to include a timestamp immediately before every Read() and Write() from Src
	ReadPrefix  []byte        // Slice of bytes that gets written to Dest immediately before every Read() from Src
	ReadSuffix  []byte        // Slice of bytes that gets written to Dest immediately after every Read() from Src
	WritePrefix []byte        // Slice of bytes that gets written to Dest immediately before every Write() to Src
	WriteSuffix []byte        // Slice of bytes that gets written to Dest immediately after every Write() to Src
}

// Logs all reads and writes on a source io.ReadWriter by writing it to a destination io.Writer.
type StreamLogger struct {
	Config *StreamLoggerConfig
}

func NewStreamLogger(c *StreamLoggerConfig) *StreamLogger {
	// TimeFormat:    "2006-01-02T15:04:05.000000",
	// Config.Src: Config.Src,
	// Config.Dest:    Config.Dest,
	return &StreamLogger{
		Config: c,
	}
}

func (l *StreamLogger) Read(p []byte) (n int, err error) {
	n, err = l.Config.Src.Read(p)
	if n > 0 {
		if len(l.Config.ReadPrefix) > 0 {
			l.Config.Dest.Write([]byte(time.Now().Format(l.Config.TimeFormat)))
			l.Config.Dest.Write(l.Config.ReadPrefix)
		}
		if n, err := l.Config.Dest.Write(p[:n]); err != nil {
			return n, err
		}
		l.Config.Dest.Write(l.Config.ReadSuffix)
	}
	return
}

func (l *StreamLogger) Write(p []byte) (n int, err error) {
	if len(p) <= 0 {
		return
	}

	if _, err := l.Config.Dest.Write([]byte(time.Now().Format(l.Config.TimeFormat))); err != nil {
		return 0, err
	}

	if _, err := l.Config.Dest.Write(l.Config.WritePrefix); err != nil {
		return 0, err
	}

	n, err = io.MultiWriter(l.Config.Src, l.Config.Dest).Write(p)

	if err != nil {
		return n, err
	}

	if _, err := l.Config.Dest.Write(l.Config.WriteSuffix); err != nil {
		return n, err
	}

	return n, nil
}
