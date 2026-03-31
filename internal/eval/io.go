package eval

import (
	"encoding/json"
	"os"
)

func LoadBenchmarkInputs(scenariosPath, predictionsPath string) ([]Scenario, []Prediction, error) {
	scenarios, err := loadScenarios(scenariosPath)
	if err != nil {
		return nil, nil, err
	}
	predictions, err := loadPredictions(predictionsPath)
	if err != nil {
		return nil, nil, err
	}
	return scenarios, predictions, nil
}

func loadScenarios(path string) ([]Scenario, error) {
	var scenarios []Scenario
	if err := loadJSON(path, &scenarios); err != nil {
		return nil, err
	}
	return scenarios, nil
}

func loadPredictions(path string) ([]Prediction, error) {
	var predictions []Prediction
	if err := loadJSON(path, &predictions); err != nil {
		return nil, err
	}
	return predictions, nil
}

func loadJSON(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
