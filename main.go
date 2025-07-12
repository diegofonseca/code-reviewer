package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/google/go-github/v58/github"
	"github.com/manifoldco/promptui"
	"golang.org/x/oauth2"
)

// Review represents a generic Pull/Merge Request.
type Review struct {
	ID          int    `json:"databaseId,omitempty"`
	Number      int    `json:"iid,omitempty"` // IID for GitLab
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	AuthorLogin string `json:"authorLogin,omitempty"` // GitHub username
	AuthorName  string `json:"authorName,omitempty"`  // GitLab username
	DiffURL     string `json:"diffUrl,omitempty"`

	// Fields for GitHub PRs (when parsing JSON from gh)
	NumberGH int `json:"number,omitempty"`
}

// CodeReviewer defines the interface for interacting with different platforms.
type CodeReviewer interface {
	ListReviews() ([]Review, error)
	GetDiff(reviewNumber int, repoPath string) (string, error)
	PlatformName() string
	GetRepoPath() string
}

// --- GitHub Implementation ---
type GitHubReviewer struct {
	client *github.Client
	owner  string
	repo   string
}

func NewGitHubReviewer() (CodeReviewer, error) {
	token, err := getGitHubToken()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	owner, repo, err := getGitRepoDetails()
	if err != nil {
		return nil, err
	}

	return &GitHubReviewer{client: client, owner: owner, repo: repo}, nil
}

func (g *GitHubReviewer) ListReviews() ([]Review, error) {
	prs, _, err := g.client.PullRequests.List(context.Background(), g.owner, g.repo, &github.PullRequestListOptions{State: "open"})
	if err != nil {
		return nil, err
	}

	var reviews []Review
	for _, pr := range prs {
		reviews = append(reviews, Review{
			ID:          int(pr.GetID()),
			Number:      pr.GetNumber(),
			Title:       pr.GetTitle(),
			Body:        pr.GetBody(),
			AuthorLogin: pr.GetUser().GetLogin(),
			DiffURL:     pr.GetDiffURL(),
			NumberGH:    pr.GetNumber(),
		})
	}
	return reviews, nil
}

func (g *GitHubReviewer) GetDiff(reviewNumber int, repoPath string) (string, error) {
	cmd := exec.Command("gh", "pr", "diff", fmt.Sprintf("%d", reviewNumber), "--repo", repoPath)
	cmd.Env = os.Environ() // Pass current environment variables
	cmd.Dir = "."          // Set working directory to current
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute `gh pr diff`: %w\nStderr: %s", err, err.(*exec.ExitError).Stderr)
	}
	return string(out), nil
}

func (g *GitHubReviewer) PlatformName() string {
	return "GitHub Pull Request"
}

func (g *GitHubReviewer) GetRepoPath() string {
	return fmt.Sprintf("%s/%s", g.owner, g.repo)
}

// --- GitLab Implementation ---
type GitLabReviewer struct {
	projectPath string
}

func NewGitLabReviewer() (CodeReviewer, error) {
	projectPath, _, err := getGitRepoDetails()
	if err != nil {
		return nil, err
	}
	return &GitLabReviewer{projectPath: projectPath}, nil
}

func (g *GitLabReviewer) ListReviews() ([]Review, error) {
	// Use `glab mr list` without --json and parse text output
	cmd := exec.Command("glab", "mr", "list", "--repo", g.projectPath)
	cmd.Env = os.Environ() // Pass current environment variables
	cmd.Dir = "."          // Set working directory to current
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute `glab mr list`: %w\nStderr: %s", err, err.(*exec.ExitError).Stderr)
	}

	var reviews []Review
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Showing") || strings.HasPrefix(line, "!") == false {
			continue // Skip empty lines, header, and non-MR lines
		}

		// Expected format: !<IID> <repo/path> <Title> (<branch>) <- (<source-branch>)
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue // Not enough parts to parse
		}

		// Extract IID
		iidStr := strings.TrimPrefix(parts[0], "!")
		iid, err := strconv.Atoi(iidStr)
		if err != nil {
			log.Printf("Warning: Could not parse IID from line: %s (Error: %v)", line, err)
			continue
		}

		// Find title and author. This is a bit fragile due to variable length fields.
		// We'll assume the title is everything between the repo path and the branch info.
		// And author is not directly available in this text output.
		// For now, we'll just use the title as is and leave author blank.
		// A more robust solution would involve `glab mr view <IID> --json` for details.

		titleParts := parts[2:] // Start from the third part (index 2) which is usually the title
		title := strings.Join(titleParts, " ")

		// Try to find the branch info to trim the title
		branchStart := strings.LastIndex(title, "(")
		branchEnd := strings.LastIndex(title, ")")
		if branchStart != -1 && branchEnd != -1 && branchEnd > branchStart {
			title = strings.TrimSpace(title[:branchStart])
		}

		reviews = append(reviews, Review{
			Number:      iid,
			Title:       title,
			Body:        "", // Not available in text output
			AuthorLogin: "", // Not available in text output
			AuthorName:  "", // Not available in text output
			DiffURL:     "", // Not available in text output
		})
	}
	return reviews, nil
}

func (g *GitLabReviewer) GetDiff(reviewNumber int, repoPath string) (string, error) {
	cmd := exec.Command("glab", "mr", "diff", fmt.Sprintf("%d", reviewNumber), "--repo", repoPath)
	cmd.Env = os.Environ() // Pass current environment variables
	cmd.Dir = "."          // Set working directory to current
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute `glab mr diff`: %w\nStderr: %s", err, err.(*exec.ExitError).Stderr)
	}
	return string(out), nil
}

func (g *GitLabReviewer) PlatformName() string {
	return "GitLab Merge Request"
}

func (g *GitLabReviewer) GetRepoPath() string {
	return g.projectPath
}

// --- Main Logic ---
func main() {
	color.Cyan("Detectando plataforma...")

	platform, err := detectPlatform()
	if err != nil {
		log.Fatalf("Erro: %v", err)
	}

	var reviewer CodeReviewer
	switch platform {
	case "github":
		color.Cyan("Plataforma GitHub detectada. Buscando Pull Requests...")
		reviewer, err = NewGitHubReviewer()
	case "gitlab":
		color.Cyan("Plataforma GitLab detectada. Buscando Merge Requests...")
		reviewer, err = NewGitLabReviewer()
	default:
		log.Fatalf("Plataforma não suportada.")
	}

	if err != nil {
		log.Fatalf("Erro ao inicializar o revisor para %s: %v", platform, err)
	}

	reviews, err := reviewer.ListReviews()
	if err != nil {
		log.Fatalf("Erro ao buscar revisões: %v", err)
	}

	if len(reviews) == 0 {
		color.Yellow("Nenhuma revisão aberta encontrada.")
		return
	}

	promptItems := make([]string, len(reviews))
	for i, r := range reviews {
		// Use NumberGH for GitHub, Number for GitLab (which is IID)
		number := r.Number
		if platform == "github" {
			number = r.NumberGH
		}
		promptItems[i] = fmt.Sprintf("#%d: %s", number, r.Title)
	}

	prompt := promptui.Select{
		Label: "Selecione uma revisão",
		Items: promptItems,
	}

	selectedIndex, _, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	selectedReview := reviews[selectedIndex]
	number := selectedReview.Number
	if platform == "github" {
		number = selectedReview.NumberGH
	}

	diff, err := reviewer.GetDiff(number, reviewer.GetRepoPath())
	if err != nil {
		log.Fatalf("Erro ao obter o diff: %v", err)
	}

	generatedPrompt := generateAgentPrompt(selectedReview, diff, reviewer.PlatformName(), platform)
	color.Green("\n\n--- Prompt para Agente de IA ---\n")
	fmt.Println(generatedPrompt)
	color.Green("\n--- Fim do Prompt ---\n\n")
}

// --- Helper Functions ---
func getGitRepoDetails() (string, string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Env = os.Environ() // Pass current environment variables
	cmd.Dir = "."          // Set working directory to current
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	url := strings.TrimSpace(string(out))
	if strings.HasPrefix(url, "git@") {
		// git@github.com:owner/repo.git -> github.com/owner/repo
		url = strings.Replace(url, ":", "/", 1)
		url = strings.TrimPrefix(url, "git@")
		url = strings.TrimSuffix(url, ".git")
	} else if strings.HasPrefix(url, "https://") {
		// https://github.com/owner/repo.git -> github.com/owner/repo
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimSuffix(url, ".git")
	}

	parts := strings.Split(url, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("URL do repositório Git inválida ou inesperada: %s", url)
	}

	// parts[0] is hostname, parts[1] is owner, parts[2] is repo
	if strings.Contains(parts[0], "github.com") {
		return parts[1], parts[2], nil
	}
	// For GitLab, the full path is needed, e.g., group/subgroup/repo
	return strings.Join(parts[1:], "/"), "", nil
}

func getGitHubToken() (string, error) {
	cmd := exec.Command("gh", "auth", "token")
	cmd.Env = os.Environ() // Pass current environment variables
	cmd.Dir = "."          // Set working directory to current
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("certifique-se de que você está logado com `gh auth login`")
	}
	return strings.TrimSpace(string(out)), nil
}

func detectPlatform() (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Env = os.Environ() // Pass current environment variables
	cmd.Dir = "."          // Set working directory to current
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("não foi possível ler a URL do repositório. Você está em um diretório git?")
	}

	url := strings.ToLower(string(out))
	if strings.Contains(url, "github.com") {
		return "github", nil
	}
	if strings.Contains(url, "gitlab.com") {
		return "gitlab", nil
	}

	return "", fmt.Errorf("plataforma não detectada. A URL do remote não contém 'github.com' ou 'gitlab.com'")
}

func generateAgentPrompt(review Review, diff, platformName, platform string) string {
	number := review.Number
	if platform == "github" {
		number = review.NumberGH
	}

	author := review.AuthorName // Use AuthorName for GitLab
	if platform == "github" {
		author = review.AuthorLogin // Use AuthorLogin for GitHub
	}

	promptTemplate := `
'''
**Solicitação de Revisão de Código**

**Plataforma:** %s
**%s:** #%d: %s
**Autor:** %s

**Descrição:**
%s

**Contexto:**
O código a seguir faz parte de um(a) %s. O objetivo desta revisão é garantir a segurança, a qualidade e a manutenibilidade do código antes de ser integrado à base de código principal.

**Instruções para o Agente de IA:**

1.  **Análise de Segurança (Security Review):**
    *   Verifique o código em busca de vulnerabilidades de segurança comuns, como:
        *   Injeção de SQL (SQL Injection)
        *   Cross-Site Scripting (XSS)
        *   Cross-Site Request Forgery (CSRF)
        *   Exposição de dados sensíveis (chaves de API, senhas, etc.).
        *   Uso de dependências inseguras.
        *   Configurações de segurança inadequadas.
    *   Forneça uma avaliação clara do risco de segurança.

2.  **Análise de Qualidade do Código (Code Quality Review):}
    *   Avalie a clareza, legibilidade e manutenibilidade do código.
    *   Verifique se o código segue as boas práticas de programação para a linguagem em questão.
    *   Identifique possíveis bugs, "code smells" ou lógica excessivamente complexa.
    *   Avalie o tratamento de erros e casos excepcionais.
    *   Sugira melhorias no código, se aplicável, com exemplos.

3.  **Conclusão e Recomendação:**
    *   Com base na sua análise, forneça uma conclusão sobre a segurança e a qualidade do(a) %s.
    *   Responda de forma inequívoca: **"É seguro fazer o merge deste(a) %s?"** (Sim/Não).
    *   Se a resposta for "Não", liste os problemas críticos que devem ser resolvidos antes do merge.

**Código para Revisão (Diff):**

` + "```diff\n" + diff + "\n```" + `

'''
`

	return fmt.Sprintf(promptTemplate, platformName, platformName, number, review.Title, author, review.Body, platformName, platformName, platformName)
}
