package forex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var apiURL = "https://api.frankfurter.app/latest?from=USD&to=CNY"

// FetchUSDCNY returns USD/CNY from Frankfurter.
func FetchUSDCNY(client *http.Client) (float64, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("frankfurter request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("frankfurter http %d", resp.StatusCode)
	}

	var parsed struct {
		Rates struct {
			CNY float64 `json:"CNY"`
		} `json:"rates"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("frankfurter parse: %w", err)
	}
	if parsed.Rates.CNY <= 0 {
		return 0, fmt.Errorf("frankfurter: invalid CNY rate")
	}
	return parsed.Rates.CNY, nil
}
