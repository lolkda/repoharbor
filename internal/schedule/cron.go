// Package schedule parses and evaluates repository synchronization schedules.
package schedule

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Default is the schedule used when a repository does not specify one.
const Default = "0 * * * *"

var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// Normalize collapses whitespace into the canonical single-space form.
func Normalize(expression string) string {
	return strings.Join(strings.Fields(expression), " ")
}

// Validate checks a standard five-field cron expression.
func Validate(expression string) error {
	expression = Normalize(expression)
	if expression == "" {
		return errors.New("cron 表达式不能为空")
	}
	if _, err := parser.Parse(expression); err != nil {
		return fmt.Errorf("cron 表达式不合法，请使用「分 时 日 月 周」五段格式: %w", err)
	}
	return nil
}

// Next returns the first scheduled time strictly after the supplied time.
func Next(expression string, after time.Time) (time.Time, error) {
	expression = Normalize(expression)
	spec, err := parser.Parse(expression)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron 表达式不合法，请使用「分 时 日 月 周」五段格式: %w", err)
	}
	return spec.Next(after), nil
}
