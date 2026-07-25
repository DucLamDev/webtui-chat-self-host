package application

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	minEveryDuration = 10 * time.Second
	maxEveryDuration = 366 * 24 * time.Hour
)

type cronField struct {
	any    bool
	values map[int]bool
}

func NextRun(schedule string, from time.Time) (time.Time, error) {
	schedule = strings.TrimSpace(strings.ToLower(schedule))
	if schedule == "" {
		return time.Time{}, fmt.Errorf("lịch chạy không được để trống")
	}

	switch schedule {
	case "@hourly":
		return from.UTC().Truncate(time.Hour).Add(time.Hour), nil
	case "@daily":
		now := from.UTC()
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC), nil
	case "@weekly":
		now := from.UTC()
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, time.UTC)
		return next, nil
	}

	if strings.HasPrefix(schedule, "@every ") {
		durationText := strings.TrimSpace(strings.TrimPrefix(schedule, "@every "))
		duration, err := time.ParseDuration(durationText)
		if err != nil || duration < minEveryDuration || duration > maxEveryDuration {
			return time.Time{}, fmt.Errorf("lịch @every phải nằm trong khoảng %s đến %s", minEveryDuration, maxEveryDuration)
		}
		return from.UTC().Add(duration), nil
	}

	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron expression cần 5 trường: phút giờ ngày tháng thứ")
	}

	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("trường phút không hợp lệ: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("trường giờ không hợp lệ: %w", err)
	}
	day, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("trường ngày không hợp lệ: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("trường tháng không hợp lệ: %w", err)
	}
	weekday, err := parseCronField(fields[4], 0, 7)
	if err != nil {
		return time.Time{}, fmt.Errorf("trường thứ không hợp lệ: %w", err)
	}
	if weekday.values[7] {
		weekday.values[0] = true
		delete(weekday.values, 7)
	}

	candidate := from.UTC().Truncate(time.Minute).Add(time.Minute)
	deadline := candidate.Add(366 * 24 * time.Hour)
	for candidate.Before(deadline) {
		cronWeekday := int(candidate.Weekday())
		if minute.matches(candidate.Minute()) &&
			hour.matches(candidate.Hour()) &&
			month.matches(int(candidate.Month())) &&
			day.matches(candidate.Day()) &&
			weekday.matches(cronWeekday) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("không tìm thấy lần chạy kế tiếp trong 366 ngày")
}

func parseCronField(value string, min int, max int) (cronField, error) {
	field := cronField{values: map[int]bool{}}
	value = strings.TrimSpace(value)
	if value == "*" {
		field.any = true
		return field, nil
	}

	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return cronField{}, fmt.Errorf("giá trị rỗng")
		}

		step := 1
		rangePart := part
		if strings.Contains(part, "/") {
			pieces := strings.Split(part, "/")
			if len(pieces) != 2 {
				return cronField{}, fmt.Errorf("step không hợp lệ")
			}
			rangePart = pieces[0]
			parsedStep, err := strconv.Atoi(pieces[1])
			if err != nil || parsedStep <= 0 {
				return cronField{}, fmt.Errorf("step phải là số dương")
			}
			step = parsedStep
		}

		start, end, err := parseCronRange(rangePart, min, max)
		if err != nil {
			return cronField{}, err
		}
		for i := start; i <= end; i += step {
			field.values[i] = true
		}
	}

	if len(field.values) == 0 {
		return cronField{}, fmt.Errorf("không có giá trị hợp lệ")
	}
	return field, nil
}

func parseCronRange(value string, min int, max int) (int, int, error) {
	if value == "*" {
		return min, max, nil
	}
	if strings.Contains(value, "-") {
		pieces := strings.Split(value, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("range không hợp lệ")
		}
		start, err := parseCronNumber(pieces[0], min, max)
		if err != nil {
			return 0, 0, err
		}
		end, err := parseCronNumber(pieces[1], min, max)
		if err != nil {
			return 0, 0, err
		}
		if start > end {
			return 0, 0, fmt.Errorf("range bắt đầu lớn hơn kết thúc")
		}
		return start, end, nil
	}
	number, err := parseCronNumber(value, min, max)
	if err != nil {
		return 0, 0, err
	}
	return number, number, nil
}

func parseCronNumber(value string, min int, max int) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("không phải số")
	}
	if number < min || number > max {
		return 0, fmt.Errorf("giá trị phải từ %d đến %d", min, max)
	}
	return number, nil
}

func (f cronField) matches(value int) bool {
	return f.any || f.values[value]
}
