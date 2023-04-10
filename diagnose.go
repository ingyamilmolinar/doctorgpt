package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	openai "github.com/sashabaranov/go-openai"
	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

type handler func(log *zap.SugaredLogger, fileName, outputDir, apiKey string, entryToDiagnose logEntry, logContext []logEntry) error 

func handleTrigger(log *zap.SugaredLogger, fileName, outputDir, apiKey string, entryToDiagnose logEntry, logContext []logEntry) error {
	err := backoff.Retry(func () error {
		// create file and write to it
		errorLocation := fileName+":"+strconv.Itoa(entryToDiagnose.LineNo)
		filename := outputDir+"/"+safeString(errorLocation)+".diagnosing"
		f, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("error creating diagnosis file: %w", err)
		}
		log.Infof("Log Line: %s", errorLocation)
		_, err = f.WriteString(fmt.Sprintf("LOG LINE:\n%s\n\n", errorLocation))
		if err != nil {
			return fmt.Errorf("error writing to diagnosis file: %w", err)
		}
		log.Infof("Prompt: %s", basePrompt)
		_, err = f.WriteString(fmt.Sprintf("BASE PROMPT:\n%s\n\n", basePrompt))
		if err != nil {
			return fmt.Errorf("error writing to diagnosis file: %w", err)
		}

		context := stringify(logContext)
		log.Infof("Context: %s", context)
		_, err = f.WriteString(fmt.Sprintf("CONTEXT:\n%s\n\n", context))
		if err != nil {
			return fmt.Errorf("error writing to diagnosis file: %w", err)
		}
		suggestion, err := suggestion(apiKey, basePrompt, context)
		if err != nil {
			return fmt.Errorf("error diagnosing using the openai API: %w", err)
		}
		log.Infof("Diagnosis: %s", suggestion)
		_, err = f.WriteString(fmt.Sprintf("DIAGNOSIS:\n%s\n", suggestion))
		if err != nil {
			return fmt.Errorf("error writing to diagnosis file: %w", err)
		}
		err = f.Close()
		if err != nil {
			return fmt.Errorf("error closing the diagnosis file: %w", err)
		}
		fullNameNoExt := strings.TrimRight(filename, ".diagnosing")
		err = os.Rename(filename, fullNameNoExt + ".diagnosed")
		if err != nil {
			return fmt.Errorf("error renaming the diagnosis file: %w", err)
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(2*time.Second), 3))
	if err != nil {
		log.Errorf("Failed to diagnose after retries: %v", err)
	}
	return err
}

func suggestion(key, basePrompt, errorMsg string) (string, error) {
	prompt := strings.Replace(basePrompt, errorPlaceholder, errorMsg, 1)
	client := openai.NewClient(key)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			// TODO: Support other models
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("error generating text from API: %v", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("chatGPT returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// TODO: Make file separator configurable
func safeString(s string) string {
	result := strings.ReplaceAll(s, " ", "-")
	result = strings.ReplaceAll(result, "/", "::")
	if len(s) > 200 {
		result = s[0:200]
	}
	return filepath.Clean(result)
}

func stringify(entries []logEntry) string {
	var result string
	for _, entry := range entries {
		result += entry.Text + "\n"
	}
	return result
}
