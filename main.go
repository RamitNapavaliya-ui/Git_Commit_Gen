package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type GeminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func main() {
	fmt.Println("🚀 Gemini Git Commit Generator")

	// Check for API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("❌ GEMINI_API_KEY environment variable is required")
	}

	fmt.Println("✅ API key found")

	// Check if in git repo
	if !isGitRepo() {
		log.Fatal("❌ Not in a git repository")
	}

	fmt.Println("✅ Git repository detected")

	// Get staged changes
	fmt.Println("📝 Getting staged changes...")
	diff, err := getStagedChanges()
	if err != nil {
		log.Fatal("❌ ", err)
	}

	fmt.Printf("✅ Found %d characters of staged changes\n", len(diff))

	// Generate commit message with Gemini
	fmt.Println("🤖 Generating commit message with Gemini...")
	message, err := generateCommitMessage(diff, apiKey)
	if err != nil {
		log.Fatal("❌ ", err)
	}

	// Show message and get confirmation
	fmt.Printf("\n💡 Generated commit message:\n%s\n\n", message)
	fmt.Print("✨ Do you want to commit with this message? (y/n): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		err := commitChanges(message)
		if err != nil {
			log.Fatal("❌ ", err)
		}
		fmt.Println("🎉 Successfully committed!")
	} else {
		fmt.Println("❌ Commit cancelled")
	}
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func getStagedChanges() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get staged changes: %w", err)
	}

	diff := string(output)
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("no staged changes found - run 'git add .' first")
	}

	return diff, nil
}

func generateCommitMessage(diff, apiKey string) (string, error) {
	prompt := fmt.Sprintf(`Generate a short git commit message for these changes:

%s

Rules:
- Use format: type: description
- Types: feat, fix, docs, style, refactor, test, chore
- Keep under 50 characters
- Be specific

Just return the commit message, nothing else:`, diff)

	reqBody := GeminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash-latest:generateContent?key=%s", apiKey)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	message := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	message = strings.Trim(message, `"'`)

	return message, nil
}

func commitChanges(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("Git output: %s", string(output))
	return nil
}
