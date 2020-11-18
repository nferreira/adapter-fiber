package fiber

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/utils"

	f "github.com/gofiber/fiber/v2"
)

func ToString(value interface{}) (*string, error) {
	result := ""
	switch v := value.(type) {
	case int:
		result = strconv.Itoa(v)
	case float32:
		result = fmt.Sprintf("%.2f", v)
	case float64:
		result = fmt.Sprintf("%.2f", v)
	case string:
		result = v
	case *string:
		result = *v
	default:
		return nil, errors.New("Headers can only be of string type")
	}
	return &result, nil
}

func GetCorrelationId(c *f.Ctx) string {
	correlationId := c.Get(CorrelationId)
	if len(strings.TrimSpace(correlationId)) == 0 {
		correlationId = utils.UUID()
	}
	return correlationId
}
