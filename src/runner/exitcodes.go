package runner

import (
	"strconv"
	"strings"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
)

func acceptedExitCode(exitCode int, successExitCodes string) bool {
	for _, accepted := range parseExitCodes(successExitCodes) {
		if exitCode == accepted {
			return true
		}
	}
	return false
}

func parseExitCodes(value string) []int {
	value = strings.TrimSpace(value)
	if value == "" {
		return []int{0}
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	result := make([]int, 0, len(fields))
	seen := map[int]bool{}
	for _, field := range fields {
		code, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	if len(result) == 0 {
		return []int{0}
	}
	return result
}

func SuccessExitCodesText(job domain.Job) string {
	codes := parseExitCodes(job.SuccessExitCodes)
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, strconv.Itoa(code))
	}
	return strings.Join(parts, ",")
}

func successExitCodesText(job domain.Job) string { return SuccessExitCodesText(job) }
