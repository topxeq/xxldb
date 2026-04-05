// Package function provides built-in functions for xxldb
package function

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/topxeq/xxldb/types"
)

// Function represents a built-in function
type Function func(args []types.Value) (types.Value, error)

// Registry holds all registered functions
var Registry = make(map[string]Function)

func init() {
	// String functions
	Register("CONCAT", fnConcat)
	Register("LENGTH", fnLength)
	Register("LEN", fnLength)
	Register("BYTE_LENGTH", fnByteLength)
	Register("OCTET_LENGTH", fnByteLength)
	Register("CHAR_LENGTH", fnLength)
	Register("CHARACTER_LENGTH", fnLength)
	Register("UPPER", fnUpper)
	Register("LOWER", fnLower)
	Register("TRIM", fnTrim)
	Register("LTRIM", fnLTrim)
	Register("RTRIM", fnRTrim)
	Register("SUBSTRING", fnSubstring)
	Register("SUBSTR", fnSubstring)
	Register("LEFT", fnLeft)
	Register("RIGHT", fnRight)
	Register("REPLACE", fnReplace)
	Register("INSTR", fnInstr)
	Register("LPAD", fnLpad)
	Register("RPAD", fnRpad)
	Register("REVERSE", fnReverse)
	Register("REPEAT", fnRepeat)

	// Numeric functions
	Register("ABS", fnAbs)
	Register("ROUND", fnRound)
	Register("FLOOR", fnFloor)
	Register("CEIL", fnCeil)
	Register("CEILING", fnCeil)
	Register("POWER", fnPower)
	Register("POW", fnPower)
	Register("SQRT", fnSqrt)
	Register("MOD", fnMod)
	Register("SIGN", fnSign)

	// Aggregate functions
	Register("COUNT", fnCount)
	Register("SUM", fnSum)
	Register("AVG", fnAvg)
	Register("MIN", fnMin)
	Register("MAX", fnMax)

	// Date/Time functions
	Register("NOW", fnNow)
	Register("CURRENT_DATE", fnCurrentDate)
	Register("CURRENT_TIME", fnCurrentTime)
	Register("DATE", fnDate)
	Register("YEAR", fnYear)
	Register("MONTH", fnMonth)
	Register("DAY", fnDay)
	Register("HOUR", fnHour)
	Register("MINUTE", fnMinute)
	Register("SECOND", fnSecond)
	Register("DATEDIFF", fnDateDiff)
	Register("DATE_ADD", fnDateAdd)
	Register("DATE_SUB", fnDateSub)
	Register("DATE_FORMAT", fnDateFormat)

	// Conversion functions
	Register("CAST", fnCast)
	Register("CONVERT", fnCast)
	Register("TO_STRING", fnToString)
	Register("TO_INT", fnToInt)
	Register("TO_FLOAT", fnToFloat)
	Register("COALESCE", fnCoalesce)
	Register("IFNULL", fnCoalesce)
	Register("NULLIF", fnNullif)

	// Conditional functions
	Register("IF", fnIf)
	Register("IIF", fnIf)

	// Utility functions
	Register("ISNULL", fnIsNull)
	Register("IS_NOT_NULL", fnIsNotNull)
	Register("TYPEOF", fnTypeof)

	// File functions
	Register("LOAD_FILE", fnLoadFile)
	Register("FILE_EXISTS", fnFileExists)
	Register("FILE_SIZE", fnFileSize)
	Register("SAVE_FILE", fnSaveFile)

	// Folder functions
	Register("LOAD_FOLDER", fnLoadFolder)
	Register("EXPORT_FOLDER", fnExportFolder)
	Register("LIST_FOLDER", fnListFolder)
	Register("FOLDER_FILES", fnFolderFiles)

	// Additional numeric functions
	Register("GCD", fnGcd)
	Register("LCM", fnLcm)
	Register("LOG", fnLog)
	Register("LOG10", fnLog10)
	Register("LOG2", fnLog2)
	Register("EXP", fnExp)
	Register("RADIANS", fnRadians)
	Register("DEGREES", fnDegrees)
	Register("SIN", fnSin)
	Register("COS", fnCos)
	Register("TAN", fnTan)
	Register("RAND", fnRand)
	Register("RANDOM", fnRand)

	// Additional string functions
	Register("CONCAT_WS", fnConcatWs)
	Register("SPACE", fnSpace)
	Register("STRCMP", fnStrcmp)

	// Additional date functions
	Register("MAKEDATE", fnMakeDate)
	Register("MAKETIME", fnMakeTime)
	Register("LAST_DAY", fnLastDay)
	Register("QUARTER", fnQuarter)
	Register("WEEK", fnWeek)
	Register("DAYOFWEEK", fnDayOfWeek)
	Register("DAYOFYEAR", fnDayOfYear)

	// UUID function
	Register("UUID", fnUuid)

	// Image functions
	Register("LOAD_IMAGE", fnLoadImage)
	Register("IMAGE_FROM_BASE64", fnImageFromBase64)
	Register("IMAGE_TO_BASE64", fnImageToBase64)
	Register("IMAGE_WIDTH", fnImageWidth)
	Register("IMAGE_HEIGHT", fnImageHeight)
	Register("IMAGE_FORMAT", fnImageFormat)
	Register("IMAGE_SIZE", fnImageSize)
	Register("IMAGE_MIME", fnImageMime)
	Register("IMAGE_FROM_HEX", fnImageFromHex)
	Register("IMAGE_TO_HEX", fnImageToHex)

	// BLOB functions
	Register("BLOB_FROM_BASE64", fnBlobFromBase64)
	Register("BLOB_TO_BASE64", fnBlobToBase64)
	Register("BLOB_FROM_HEX", fnBlobFromHex)
	Register("BLOB_TO_HEX", fnBlobToHex)
}

// Register registers a function
func Register(name string, fn Function) {
	Registry[strings.ToUpper(name)] = fn
}

// Get retrieves a function by name
func Get(name string) (Function, bool) {
	fn, ok := Registry[strings.ToUpper(name)]
	return fn, ok
}

// Call calls a function by name
func Call(name string, args []types.Value) (types.Value, error) {
	fn, ok := Get(name)
	if !ok {
		return types.Value{}, fmt.Errorf("unknown function: %s", name)
	}
	return fn(args)
}

// ==================== String Functions ====================

func fnConcat(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("CONCAT requires at least 1 argument")
	}
	var sb strings.Builder
	for _, arg := range args {
		if !arg.IsNull {
			sb.WriteString(arg.ToString())
		}
	}
	return types.NewStringValue(sb.String()), nil
}

func fnLength(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LENGTH requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	// Return character count (runes), not byte count
	return types.NewIntValue(int64(utf8.RuneCountInString(s))), nil
}

func fnByteLength(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("BYTE_LENGTH requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	// Return byte count (UTF-8 encoded)
	return types.NewIntValue(int64(len(s))), nil
}

func fnUpper(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("UPPER requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	return types.NewStringValue(strings.ToUpper(args[0].ToString())), nil
}

func fnLower(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LOWER requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	return types.NewStringValue(strings.ToLower(args[0].ToString())), nil
}

func fnTrim(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("TRIM requires at least 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	if len(args) > 1 {
		chars := args[1].ToString()
		return types.NewStringValue(strings.Trim(s, chars)), nil
	}
	return types.NewStringValue(strings.TrimSpace(s)), nil
}

func fnLTrim(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("LTRIM requires at least 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	if len(args) > 1 {
		chars := args[1].ToString()
		return types.NewStringValue(strings.TrimLeft(s, chars)), nil
	}
	return types.NewStringValue(strings.TrimLeft(s, " ")), nil
}

func fnRTrim(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("RTRIM requires at least 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	if len(args) > 1 {
		chars := args[1].ToString()
		return types.NewStringValue(strings.TrimRight(s, chars)), nil
	}
	return types.NewStringValue(strings.TrimRight(s, " ")), nil
}

func fnSubstring(args []types.Value) (types.Value, error) {
	if len(args) < 2 {
		return types.Value{}, fmt.Errorf("SUBSTRING requires at least 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	start, _ := args[1].ToInt64()
	start-- // SQL is 1-indexed

	// Convert to runes for proper Unicode handling
	runes := []rune(s)
	runeLen := int64(len(runes))
	if len(args) > 2 {
		runeLen, _ = args[2].ToInt64()
	}

	if start < 0 {
		start = 0
	}
	end := int(start + runeLen)
	if end > len(runes) {
		end = len(runes)
	}

	if int(start) > len(runes) {
		return types.NewStringValue(""), nil
	}

	return types.NewStringValue(string(runes[start:end])), nil
}

func fnLeft(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("LEFT requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	n, _ := args[1].ToInt64()

	// Convert to runes for proper Unicode handling
	runes := []rune(s)
	if n > int64(len(runes)) {
		n = int64(len(runes))
	}
	return types.NewStringValue(string(runes[:n])), nil
}

func fnRight(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("RIGHT requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	n, _ := args[1].ToInt64()

	// Convert to runes for proper Unicode handling
	runes := []rune(s)
	if n > int64(len(runes)) {
		n = int64(len(runes))
	}
	return types.NewStringValue(string(runes[len(runes)-int(n):])), nil
}

func fnReplace(args []types.Value) (types.Value, error) {
	if len(args) != 3 {
		return types.Value{}, fmt.Errorf("REPLACE requires 3 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	old := args[1].ToString()
	new := args[2].ToString()
	return types.NewStringValue(strings.ReplaceAll(s, old, new)), nil
}

func fnInstr(args []types.Value) (types.Value, error) {
	if len(args) < 2 {
		return types.Value{}, fmt.Errorf("INSTR requires at least 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	substr := args[1].ToString()

	// Find byte position
	bytePos := strings.Index(s, substr)
	if bytePos == -1 {
		return types.NewIntValue(0), nil
	}

	// Convert byte position to character position
	charPos := utf8.RuneCountInString(s[:bytePos]) + 1
	return types.NewIntValue(int64(charPos)), nil
}

func fnLpad(args []types.Value) (types.Value, error) {
	if len(args) < 3 {
		return types.Value{}, fmt.Errorf("LPAD requires at least 3 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	length, _ := args[1].ToInt64()
	pad := args[2].ToString()

	// Convert to runes for proper Unicode handling
	runes := []rune(s)
	padRunes := []rune(pad)

	for int64(len(runes)) < length {
		runes = append(padRunes, runes...)
	}
	if int64(len(runes)) > length {
		runes = runes[int64(len(runes))-length:]
	}
	return types.NewStringValue(string(runes)), nil
}

func fnRpad(args []types.Value) (types.Value, error) {
	if len(args) < 3 {
		return types.Value{}, fmt.Errorf("RPAD requires at least 3 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	length, _ := args[1].ToInt64()
	pad := args[2].ToString()

	// Convert to runes for proper Unicode handling
	runes := []rune(s)
	padRunes := []rune(pad)

	for int64(len(runes)) < length {
		runes = append(runes, padRunes...)
	}
	if int64(len(runes)) > length {
		runes = runes[:length]
	}
	return types.NewStringValue(string(runes)), nil
}

func fnReverse(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("REVERSE requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return types.NewStringValue(string(runes)), nil
}

func fnRepeat(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("REPEAT requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	n, _ := args[1].ToInt64()
	return types.NewStringValue(strings.Repeat(s, int(n))), nil
}

// ==================== Numeric Functions ====================

func fnAbs(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("ABS requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewFloatValue(math.Abs(f)), nil
}

func fnRound(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("ROUND requires at least 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	precision := 0
	if len(args) > 1 {
		p, _ := args[1].ToInt64()
		precision = int(p)
	}

	mult := math.Pow10(precision)
	result := math.Round(f*mult) / mult
	return types.NewFloatValue(result), nil
}

func fnFloor(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("FLOOR requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(math.Floor(f))), nil
}

func fnCeil(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("CEIL requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(math.Ceil(f))), nil
}

func fnPower(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("POWER requires 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	base, _ := args[0].ToFloat64()
	exp, _ := args[1].ToFloat64()
	return types.NewFloatValue(math.Pow(base, exp)), nil
}

func fnSqrt(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("SQRT requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	if f < 0 {
		return types.Value{}, fmt.Errorf("cannot take square root of negative number")
	}
	return types.NewFloatValue(math.Sqrt(f)), nil
}

func fnMod(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("MOD requires 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	a, _ := args[0].ToInt64()
	b, _ := args[1].ToInt64()
	if b == 0 {
		return types.Value{}, fmt.Errorf("division by zero")
	}
	return types.NewIntValue(a % b), nil
}

func fnSign(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("SIGN requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	if f > 0 {
		return types.NewIntValue(1), nil
	} else if f < 0 {
		return types.NewIntValue(-1), nil
	}
	return types.NewIntValue(0), nil
}

// ==================== Aggregate Functions ====================

func fnCount(args []types.Value) (types.Value, error) {
	count := int64(0)
	for _, arg := range args {
		if !arg.IsNull {
			count++
		}
	}
	return types.NewIntValue(count), nil
}

func fnSum(args []types.Value) (types.Value, error) {
	var sum float64
	for _, arg := range args {
		if !arg.IsNull {
			f, err := arg.ToFloat64()
			if err != nil {
				return types.Value{}, err
			}
			sum += f
		}
	}
	return types.NewFloatValue(sum), nil
}

func fnAvg(args []types.Value) (types.Value, error) {
	var sum float64
	var count int64
	for _, arg := range args {
		if !arg.IsNull {
			f, err := arg.ToFloat64()
			if err != nil {
				return types.Value{}, err
			}
			sum += f
			count++
		}
	}
	if count == 0 {
		return types.NewNullValue(), nil
	}
	return types.NewFloatValue(sum / float64(count)), nil
}

func fnMin(args []types.Value) (types.Value, error) {
	if len(args) == 0 {
		return types.NewNullValue(), nil
	}
	min := args[0]
	for _, arg := range args[1:] {
		if !arg.IsNull && arg.Compare(min) < 0 {
			min = arg
		}
	}
	return min, nil
}

func fnMax(args []types.Value) (types.Value, error) {
	if len(args) == 0 {
		return types.NewNullValue(), nil
	}
	max := args[0]
	for _, arg := range args[1:] {
		if !arg.IsNull && arg.Compare(max) > 0 {
			max = arg
		}
	}
	return max, nil
}

// ==================== Date/Time Functions ====================

func fnNow(args []types.Value) (types.Value, error) {
	return types.NewDatetimeValue(time.Now()), nil
}

func fnCurrentDate(args []types.Value) (types.Value, error) {
	return types.NewDateValue(time.Now()), nil
}

func fnCurrentTime(args []types.Value) (types.Value, error) {
	return types.NewDatetimeValue(time.Now()), nil
}

func fnDate(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("DATE requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	s := args[0].ToString()
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return types.Value{}, fmt.Errorf("invalid date format: %s", s)
	}
	return types.NewDateValue(t), nil
}

func fnYear(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("YEAR requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.Year())), nil
}

func fnMonth(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("MONTH requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.Month())), nil
}

func fnDay(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("DAY requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.Day())), nil
}

func fnHour(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("HOUR requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.Hour())), nil
}

func fnMinute(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("MINUTE requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.Minute())), nil
}

func fnSecond(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("SECOND requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.Second())), nil
}

func fnDateDiff(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("DATEDIFF requires 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	t1, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	t2, err := args[1].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	days := int64(t1.Sub(t2).Hours() / 24)
	return types.NewIntValue(days), nil
}

func fnDateAdd(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("DATE_ADD requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	days, _ := args[1].ToInt64()
	return types.NewDateValue(t.AddDate(0, 0, int(days))), nil
}

func fnDateSub(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("DATE_SUB requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	days, _ := args[1].ToInt64()
	return types.NewDateValue(t.AddDate(0, 0, -int(days))), nil
}

func fnDateFormat(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("DATE_FORMAT requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}

	layout := args[1].ToString()
	// Convert SQL-style format to Go format
	layout = strings.ReplaceAll(layout, "%Y", "2006")
	layout = strings.ReplaceAll(layout, "%m", "01")
	layout = strings.ReplaceAll(layout, "%d", "02")
	layout = strings.ReplaceAll(layout, "%H", "15")
	layout = strings.ReplaceAll(layout, "%i", "04")
	layout = strings.ReplaceAll(layout, "%s", "05")

	return types.NewStringValue(t.Format(layout)), nil
}

// ==================== Conversion Functions ====================

func fnCast(args []types.Value) (types.Value, error) {
	if len(args) < 2 {
		return types.Value{}, fmt.Errorf("CAST requires 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	targetType := strings.ToUpper(args[1].ToString())
	switch targetType {
	case "INT", "INTEGER":
		n, err := args[0].ToInt64()
		if err != nil {
			return types.Value{}, err
		}
		return types.NewIntValue(n), nil
	case "FLOAT", "DOUBLE":
		f, err := args[0].ToFloat64()
		if err != nil {
			return types.Value{}, err
		}
		return types.NewFloatValue(f), nil
	case "VARCHAR", "CHAR", "STRING", "TEXT":
		return types.NewStringValue(args[0].ToString()), nil
	default:
		return types.Value{}, fmt.Errorf("unsupported cast type: %s", targetType)
	}
}

func fnToString(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("TO_STRING requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	return types.NewStringValue(args[0].ToString()), nil
}

func fnToInt(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("TO_INT requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	n, err := args[0].ToInt64()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(n), nil
}

func fnToFloat(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("TO_FLOAT requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	f, err := args[0].ToFloat64()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewFloatValue(f), nil
}

func fnCoalesce(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("COALESCE requires at least 1 argument")
	}
	for _, arg := range args {
		if !arg.IsNull {
			return arg, nil
		}
	}
	return types.NewNullValue(), nil
}

func fnNullif(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("NULLIF requires 2 arguments")
	}
	if args[0].Compare(args[1]) == 0 {
		return types.NewNullValue(), nil
	}
	return args[0], nil
}

// ==================== Conditional Functions ====================

func fnIf(args []types.Value) (types.Value, error) {
	if len(args) != 3 {
		return types.Value{}, fmt.Errorf("IF requires 3 arguments")
	}
	cond, err := args[0].ToInt64()
	if err != nil {
		return types.Value{}, err
	}
	if cond != 0 {
		return args[1], nil
	}
	return args[2], nil
}

// ==================== Utility Functions ====================

func fnIsNull(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("ISNULL requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewIntValue(1), nil
	}
	return types.NewIntValue(0), nil
}

func fnIsNotNull(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("IS_NOT_NULL requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewIntValue(0), nil
	}
	return types.NewIntValue(1), nil
}

func fnTypeof(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("TYPEOF requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewStringValue("NULL"), nil
	}
	return types.NewStringValue(args[0].Type.String()), nil
}

// ==================== File Functions ====================

func fnLoadFile(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LOAD_FILE requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	filePath := args[0].ToString()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return types.NewNullValue(), nil
	}
	return types.NewBlobValue(data), nil
}

func fnFileExists(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("FILE_EXISTS requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewIntValue(0), nil
	}

	filePath := args[0].ToString()
	if _, err := os.Stat(filePath); err == nil {
		return types.NewIntValue(1), nil
	}
	return types.NewIntValue(0), nil
}

func fnFileSize(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("FILE_SIZE requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	filePath := args[0].ToString()
	info, err := os.Stat(filePath)
	if err != nil {
		return types.NewNullValue(), nil
	}
	return types.NewIntValue(info.Size()), nil
}

func fnSaveFile(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("SAVE_FILE requires 2 arguments: path and content")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewIntValue(0), nil
	}

	filePath := args[0].ToString()
	var data []byte
	if args[1].Type == types.TypeBlob {
		if b, ok := args[1].Data.([]byte); ok {
			data = b
		} else {
			data = []byte(args[1].ToString())
		}
	} else {
		data = []byte(args[1].ToString())
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return types.NewIntValue(0), nil
	}
	return types.NewIntValue(1), nil
}


// ==================== Folder Functions ====================

// FolderEntry represents a file or folder entry
type FolderEntry struct {
	Name     string        `json:"name"`
	IsDir    bool          `json:"is_dir"`
	Size     int64         `json:"size,omitempty"`
	ModTime  string        `json:"mod_time,omitempty"`
	Content  []byte        `json:"content,omitempty"`
	Children []FolderEntry `json:"children,omitempty"`
}

func fnLoadFolder(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LOAD_FOLDER requires 1 argument: folder path")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	folderPath := args[0].ToString()

	// Read folder structure
	entries, err := readFolderRecursive(folderPath)
	if err != nil {
		return types.NewNullValue(), nil
	}

	// Create root entry
	root := FolderEntry{
		Name:     filepath.Base(folderPath),
		IsDir:    true,
		Children: entries,
	}

	// Serialize to JSON
	data, err := json.Marshal(root)
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewBlobValue(data), nil
}

func readFolderRecursive(path string) ([]FolderEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []FolderEntry
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		info, _ := entry.Info()

		fe := FolderEntry{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		}

		if entry.IsDir() {
			children, err := readFolderRecursive(fullPath)
			if err == nil {
				fe.Children = children
			}
		} else {
			// Read file content (limit size to avoid memory issues)
			if info.Size() < 10*1024*1024 { // 10MB limit per file
				data, err := os.ReadFile(fullPath)
				if err == nil {
					fe.Content = data
				}
			}
		}

		result = append(result, fe)
	}

	return result, nil
}

func fnExportFolder(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("EXPORT_FOLDER requires 2 arguments: folder_data and target_path")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewIntValue(0), nil
	}

	var data []byte
	if args[0].Type == types.TypeBlob {
		data = args[0].Data.([]byte)
	} else {
		data = []byte(args[0].ToString())
	}

	targetPath := args[1].ToString()

	var root FolderEntry
	if err := json.Unmarshal(data, &root); err != nil {
		return types.NewIntValue(0), nil
	}

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return types.NewIntValue(0), nil
	}

	if err := writeFolderRecursive(targetPath, root.Children); err != nil {
		return types.NewIntValue(0), nil
	}

	return types.NewIntValue(1), nil
}

func writeFolderRecursive(basePath string, entries []FolderEntry) error {
	for _, entry := range entries {
		fullPath := filepath.Join(basePath, entry.Name)

		if entry.IsDir {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return err
			}
			if len(entry.Children) > 0 {
				if err := writeFolderRecursive(fullPath, entry.Children); err != nil {
					return err
				}
			}
		} else {
			if len(entry.Content) > 0 {
				if err := os.WriteFile(fullPath, entry.Content, 0644); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func fnListFolder(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LIST_FOLDER requires 1 argument: folder_data")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	var data []byte
	if args[0].Type == types.TypeBlob {
		data = args[0].Data.([]byte)
	} else {
		data = []byte(args[0].ToString())
	}

	var root FolderEntry
	if err := json.Unmarshal(data, &root); err != nil {
		return types.NewNullValue(), nil
	}

	var listing strings.Builder
	listFolderRecursive(&listing, root.Children, 0)

	return types.NewStringValue(listing.String()), nil
}

func listFolderRecursive(sb *strings.Builder, entries []FolderEntry, indent int) {
	prefix := strings.Repeat("  ", indent)
	for _, entry := range entries {
		if entry.IsDir {
			sb.WriteString(fmt.Sprintf("%s[d] %s/\n", prefix, entry.Name))
			if len(entry.Children) > 0 {
				listFolderRecursive(sb, entry.Children, indent+1)
			}
		} else {
			size := formatSize(entry.Size)
			sb.WriteString(fmt.Sprintf("%s[f] %s (%s)\n", prefix, entry.Name, size))
		}
	}
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1fGB", float64(size)/(1024*1024*1024))
}

func fnFolderFiles(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("FOLDER_FILES requires 1 argument: folder_data")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	var data []byte
	if args[0].Type == types.TypeBlob {
		data = args[0].Data.([]byte)
	} else {
		data = []byte(args[0].ToString())
	}

	var root FolderEntry
	if err := json.Unmarshal(data, &root); err != nil {
		return types.NewNullValue(), nil
	}

	count := countFiles(root.Children)
	return types.NewIntValue(int64(count)), nil
}

func countFiles(entries []FolderEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.IsDir {
			count += countFiles(entry.Children)
		} else {
			count++
		}
	}
	return count
}

// IsAggregate checks if a function is an aggregate function
func IsAggregate(name string) bool {
	upper := strings.ToUpper(name)
	switch upper {
	case "COUNT", "SUM", "AVG", "MIN", "MAX":
		return true
	default:
		return false
	}
}

// HasAggregate checks if an expression contains aggregate functions
func HasAggregate(expr string) bool {
	re := regexp.MustCompile(`(?i)\b(COUNT|SUM|AVG|MIN|MAX)\s*\(`)
	return re.MatchString(expr)
}

// ==================== Additional Numeric Functions ====================

func fnGcd(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("GCD requires 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	a, _ := args[0].ToInt64()
	b, _ := args[1].ToInt64()
	return types.NewIntValue(gcd(a, b)), nil
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func fnLcm(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("LCM requires 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	a, _ := args[0].ToInt64()
	b, _ := args[1].ToInt64()
	return types.NewIntValue(lcm(a, b)), nil
}

func lcm(a, b int64) int64 {
	return a * b / gcd(a, b)
}

func fnLog(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("LOG requires at least 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	if len(args) == 1 {
		return types.NewFloatValue(math.Log(x)), nil
	}
	// LOG(base, x) - logarithm with specified base
	base, _ := args[1].ToFloat64()
	return types.NewFloatValue(math.Log(x) / math.Log(base)), nil
}

func fnLog10(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LOG10 requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(math.Log10(x)), nil
}

func fnLog2(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LOG2 requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(math.Log2(x)), nil
}

func fnExp(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("EXP requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(math.Exp(x)), nil
}

func fnRadians(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("RADIANS requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(x * math.Pi / 180), nil
}

func fnDegrees(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("DEGREES requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(x * 180 / math.Pi), nil
}

func fnSin(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("SIN requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(math.Sin(x)), nil
}

func fnCos(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("COS requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(math.Cos(x)), nil
}

func fnTan(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("TAN requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	x, _ := args[0].ToFloat64()
	return types.NewFloatValue(math.Tan(x)), nil
}

func fnRand(args []types.Value) (types.Value, error) {
	return types.NewFloatValue(rand.Float64()), nil
}

// ==================== Additional String Functions ====================

func fnConcatWs(args []types.Value) (types.Value, error) {
	if len(args) < 2 {
		return types.Value{}, fmt.Errorf("CONCAT_WS requires at least 2 arguments")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	sep := args[0].ToString()
	var parts []string
	for _, arg := range args[1:] {
		if !arg.IsNull {
			parts = append(parts, arg.ToString())
		}
	}
	return types.NewStringValue(strings.Join(parts, sep)), nil
}

func fnSpace(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("SPACE requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	n, _ := args[0].ToInt64()
	if n < 0 {
		n = 0
	}
	return types.NewStringValue(strings.Repeat(" ", int(n))), nil
}

func fnStrcmp(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("STRCMP requires 2 arguments")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	s1 := args[0].ToString()
	s2 := args[1].ToString()
	if s1 < s2 {
		return types.NewIntValue(-1), nil
	} else if s1 > s2 {
		return types.NewIntValue(1), nil
	}
	return types.NewIntValue(0), nil
}

// ==================== Additional Date Functions ====================

func fnMakeDate(args []types.Value) (types.Value, error) {
	if len(args) != 2 {
		return types.Value{}, fmt.Errorf("MAKEDATE requires 2 arguments: year and dayofyear")
	}
	if args[0].IsNull || args[1].IsNull {
		return types.NewNullValue(), nil
	}
	year, _ := args[0].ToInt64()
	dayOfYear, _ := args[1].ToInt64()
	t := time.Date(int(year), 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(dayOfYear-1))
	return types.NewDateValue(t), nil
}

func fnMakeTime(args []types.Value) (types.Value, error) {
	if len(args) != 3 {
		return types.Value{}, fmt.Errorf("MAKETIME requires 3 arguments: hour, minute, second")
	}
	if args[0].IsNull || args[1].IsNull || args[2].IsNull {
		return types.NewNullValue(), nil
	}
	hour, _ := args[0].ToInt64()
	minute, _ := args[1].ToInt64()
	second, _ := args[2].ToFloat64()
	sec := int(second)
	nsec := int((second - float64(sec)) * 1e9)
	t := time.Date(0, 1, 1, int(hour), int(minute), sec, nsec, time.UTC)
	return types.NewDatetimeValue(t), nil
}

func fnLastDay(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LAST_DAY requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	// Get first day of next month, then subtract one day
	nextMonth := t.AddDate(0, 1, -t.Day()+1)
	lastDay := nextMonth.AddDate(0, 0, -1)
	return types.NewDateValue(lastDay), nil
}

func fnQuarter(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("QUARTER requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	month := t.Month()
	quarter := (month-1)/3 + 1
	return types.NewIntValue(int64(quarter)), nil
}

func fnWeek(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("WEEK requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	_, week := t.ISOWeek()
	return types.NewIntValue(int64(week)), nil
}

func fnDayOfWeek(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("DAYOFWEEK requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	// Sunday = 1, Saturday = 7
	return types.NewIntValue(int64(t.Weekday() + 1)), nil
}

func fnDayOfYear(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("DAYOFYEAR requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}
	t, err := args[0].ToTime()
	if err != nil {
		return types.Value{}, err
	}
	return types.NewIntValue(int64(t.YearDay())), nil
}

// ==================== UUID Function ====================

func fnUuid(args []types.Value) (types.Value, error) {
	uuid := make([]byte, 16)
	rand.Read(uuid)
	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return types.NewStringValue(fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])), nil
}

// ==================== Image Functions ====================

// ImageInfo holds image metadata
type ImageInfo struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
	Size   int    `json:"size"`
	Mime   string `json:"mime"`
}

// formatToMime maps image format to MIME type
var formatToMime = map[string]string{
	"png":  "image/png",
	"jpeg": "image/jpeg",
	"jpg":  "image/jpeg",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"tiff": "image/tiff",
}

// parseImageInfo extracts image metadata from binary data
func parseImageInfo(data []byte) (*ImageInfo, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}

	info := &ImageInfo{
		Size: len(data),
	}

	// Try to decode image to get dimensions
	reader := strings.NewReader(string(data))
	img, format, err := image.DecodeConfig(reader)
	if err != nil {
		// If decode fails, try to detect format from header
		info.Format = detectImageFormat(data)
		if info.Format == "" {
			info.Format = "unknown"
		}
	} else {
		info.Width = img.Width
		info.Height = img.Height
		info.Format = format
	}

	if mime, ok := formatToMime[info.Format]; ok {
		info.Mime = mime
	} else {
		info.Mime = "application/octet-stream"
	}

	return info, nil
}

// detectImageFormat detects image format from magic bytes
func detectImageFormat(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "png"
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}
	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "gif"
	}
	// BMP: 42 4D
	if data[0] == 0x42 && data[1] == 0x4D {
		return "bmp"
	}
	// TIFF: 49 49 or 4D 4D
	if (data[0] == 0x49 && data[1] == 0x49) || (data[0] == 0x4D && data[1] == 0x4D) {
		return "tiff"
	}
	// WebP: 52 49 46 46 ... 57 45 42 50
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if len(data) > 11 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "webp"
		}
	}

	return ""
}

// getImageData extracts binary data from IMAGE or BLOB value
func getImageData(val types.Value) ([]byte, error) {
	if val.IsNull {
		return nil, fmt.Errorf("image is null")
	}

	if val.Type == types.TypeImage || val.Type == types.TypeBlob {
		if data, ok := val.Data.([]byte); ok {
			return data, nil
		}
	}

	return nil, fmt.Errorf("expected IMAGE or BLOB type, got %s", val.Type)
}

func fnLoadImage(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("LOAD_IMAGE requires 1 argument: file path")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	filePath := args[0].ToString()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewImageValue(data), nil
}

func fnImageFromBase64(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("IMAGE_FROM_BASE64 requires at least 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	input := args[0].ToString()

	// Handle Data URI format: data:image/png;base64,iVBORw0KGgo...
	if strings.HasPrefix(input, "data:image/") {
		// Extract base64 part after comma
		if idx := strings.Index(input, ","); idx != -1 {
			input = input[idx+1:]
		}
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		// Try URL encoding
		data, err = base64.URLEncoding.DecodeString(input)
		if err != nil {
			return types.Value{}, fmt.Errorf("invalid base64 data: %v", err)
		}
	}

	return types.NewImageValue(data), nil
}

func fnImageToBase64(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("IMAGE_TO_BASE64 requires at least 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	// Check if user wants Data URI format
	if len(args) > 1 && strings.ToLower(args[1].ToString()) == "datauri" {
		info, err := parseImageInfo(data)
		if err == nil {
			encoded = fmt.Sprintf("data:%s;base64,%s", info.Mime, encoded)
		}
	}

	return types.NewStringValue(encoded), nil
}

func fnImageWidth(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("IMAGE_WIDTH requires 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	info, err := parseImageInfo(data)
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewIntValue(int64(info.Width)), nil
}

func fnImageHeight(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("IMAGE_HEIGHT requires 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	info, err := parseImageInfo(data)
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewIntValue(int64(info.Height)), nil
}

func fnImageFormat(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("IMAGE_FORMAT requires 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	info, err := parseImageInfo(data)
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewStringValue(info.Format), nil
}

func fnImageSize(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("IMAGE_SIZE requires 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewIntValue(int64(len(data))), nil
}

func fnImageMime(args []types.Value) (types.Value, error) {
	if len(args) != 1 {
		return types.Value{}, fmt.Errorf("IMAGE_MIME requires 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	info, err := parseImageInfo(data)
	if err != nil {
		return types.NewNullValue(), nil
	}

	return types.NewStringValue(info.Mime), nil
}

// fnImageFromHex creates an IMAGE from a hex string
func fnImageFromHex(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("IMAGE_FROM_HEX requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	input := args[0].ToString()

	// Remove optional 0x prefix
	input = strings.TrimPrefix(input, "0x")
	input = strings.TrimPrefix(input, "0X")

	// Remove any whitespace
	input = strings.ReplaceAll(input, " ", "")
	input = strings.ReplaceAll(input, "\n", "")
	input = strings.ReplaceAll(input, "\r", "")
	input = strings.ReplaceAll(input, "\t", "")

	// Validate hex string length (must be even)
	if len(input)%2 != 0 {
		return types.Value{}, fmt.Errorf("invalid hex string: length must be even")
	}

	// Decode hex
	data, err := hex.DecodeString(input)
	if err != nil {
		return types.Value{}, fmt.Errorf("invalid hex data: %v", err)
	}

	return types.NewImageValue(data), nil
}

// fnImageToHex converts an IMAGE to a hex string
func fnImageToHex(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("IMAGE_TO_HEX requires 1 argument")
	}

	data, err := getImageData(args[0])
	if err != nil {
		return types.NewNullValue(), nil
	}

	// Encode to hex (lowercase)
	encoded := hex.EncodeToString(data)

	return types.NewStringValue(encoded), nil
}

// fnBlobFromBase64 creates a BLOB from a base64 string
func fnBlobFromBase64(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("BLOB_FROM_BASE64 requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	input := args[0].ToString()

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		// Try URL encoding
		data, err = base64.URLEncoding.DecodeString(input)
		if err != nil {
			return types.Value{}, fmt.Errorf("invalid base64 data: %v", err)
		}
	}

	return types.NewBlobValue(data), nil
}

// fnBlobToBase64 converts a BLOB to a base64 string
func fnBlobToBase64(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("BLOB_TO_BASE64 requires 1 argument")
	}

	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	var data []byte
	switch v := args[0].Data.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return types.Value{}, fmt.Errorf("BLOB_TO_BASE64 requires a BLOB argument")
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return types.NewStringValue(encoded), nil
}

// fnBlobFromHex creates a BLOB from a hex string
func fnBlobFromHex(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("BLOB_FROM_HEX requires 1 argument")
	}
	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	input := args[0].ToString()

	// Remove optional 0x prefix
	input = strings.TrimPrefix(input, "0x")
	input = strings.TrimPrefix(input, "0X")

	// Remove any whitespace
	input = strings.ReplaceAll(input, " ", "")
	input = strings.ReplaceAll(input, "\n", "")
	input = strings.ReplaceAll(input, "\r", "")
	input = strings.ReplaceAll(input, "\t", "")

	// Validate hex string length (must be even)
	if len(input)%2 != 0 {
		return types.Value{}, fmt.Errorf("invalid hex string: length must be even")
	}

	// Decode hex
	data, err := hex.DecodeString(input)
	if err != nil {
		return types.Value{}, fmt.Errorf("invalid hex data: %v", err)
	}

	return types.NewBlobValue(data), nil
}

// fnBlobToHex converts a BLOB to a hex string
func fnBlobToHex(args []types.Value) (types.Value, error) {
	if len(args) < 1 {
		return types.Value{}, fmt.Errorf("BLOB_TO_HEX requires 1 argument")
	}

	if args[0].IsNull {
		return types.NewNullValue(), nil
	}

	var data []byte
	switch v := args[0].Data.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return types.Value{}, fmt.Errorf("BLOB_TO_HEX requires a BLOB argument")
	}

	// Encode to hex (lowercase)
	encoded := hex.EncodeToString(data)
	return types.NewStringValue(encoded), nil
}

// Unused imports workaround - these are needed for image decoding
var (
	_ = png.Decode
	_ = jpeg.Decode
	_ = gif.Decode
	_ = io.EOF
)
