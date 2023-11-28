// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

type messageFormatter struct {
	withPrefix bool
}

func (f *messageFormatter) Format(entry *log.Entry) ([]byte, error) {
	data := make(log.Fields)
	for k, v := range entry.Data {
		data[k] = v
	}

	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	if f.withPrefix {
		fmt.Fprintf(b, "[%s] ", strings.ToUpper(entry.Level.String())[0:4])
	}

	if entry.Message != "" {
		fmt.Fprintf(b, "%s ", entry.Message)
	}
	for k, v := range data {
		fmt.Fprintf(b, `%s="%v" `, k, v)
	}

	return b.Bytes(), nil
}

var messageLogFormat = messageFormatter{withPrefix: true}

type messageLogHook struct {
	e *Extractor
}

func (h *messageLogHook) Levels() []log.Level {
	return log.AllLevels
}

func (h *messageLogHook) Fire(entry *log.Entry) error {
	b, _ := messageLogFormat.Format(entry)
	h.e.Logs = append(h.e.Logs, strings.TrimSpace(string(b)))
	if entry.Level <= log.ErrorLevel {
		h.e.errors = append(h.e.errors, errors.New(entry.Message))
	}

	if entry.Level <= log.StandardLogger().Level {
		msg, _ := entry.Logger.Formatter.Format(entry)
		if _, err := log.StandardLogger().Out.Write(msg); err != nil {
			return err
		}
	}

	return nil
}
