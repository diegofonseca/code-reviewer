# Code Review CLI (Multi-Platform)

Uma ferramenta de linha de comando que funciona tanto para **GitHub** quanto para **GitLab**. Ela detecta automaticamente a plataforma do seu projeto, lista os Pull/Merge Requests abertos e gera um prompt de revisão de código para ser usado com agentes de IA como o Copilot.

## Como Funciona

A ferramenta lê a URL do seu `remote` Git (`origin`) para detectar se o projeto está no GitHub ou no GitLab. Com base na plataforma detectada, ela usa a CLI apropriada (`gh` ou `glab`) para se autenticar e interagir com a API correspondente.

## Pré-requisitos

- **Go:** Você precisa ter o Go instalado e configurado no seu ambiente. ([Instruções de Instalação](https://golang.org/doc/install))

- **CLI da sua Plataforma:** Você precisa ter a CLI correspondente à sua plataforma de hospedagem de código instalada e **autenticada**.

  - **Para projetos GitHub:**
    - Instale a **GitHub CLI (`gh`)**. ([Instruções](https://github.com/cli/cli#installation))
    - Autentique-se com: `gh auth login`

  - **Para projetos GitLab:**
    - Instale a **GitLab CLI (`glab`)**. ([Instruções](https://gitlab.com/gitlab-org/cli#installation))
    - Autentique-se com: `glab auth login`

## Instalação e Uso

1.  **Clone o repositório (ou use no seu projeto existente):**

    ```bash
    # Exemplo para GitHub
    git clone https://github.com/seu-usuario/seu-repositorio.git
    cd seu-repositorio
    ```

2.  **Instale as dependências:**

    No diretório do projeto, execute o seguinte comando para baixar as dependências necessárias para ambas as plataformas:

    ```bash
    go mod tidy
    ```

3.  **Compile o binário:**

    Para criar o executável `code-review`, execute:

    ```bash
    go build -o code-review .
    ```

4.  **Execute a ferramenta:**

    Certifique-se de estar no diretório de um projeto Git com um `remote` configurado para o GitHub ou GitLab.

    ```bash
    ./code-review
    ```

    A ferramenta irá:
    - Detectar automaticamente se você está em um repositório GitHub ou GitLab.
    - Listar os Pull Requests ou Merge Requests abertos.
    - Permitir que você selecione uma revisão de forma interativa.
    - Gerar um prompt detalhado no terminal.

5.  **Use o Prompt Gerado:**

    Copie o prompt gerado e cole-o no seu agente de código preferido para obter uma análise detalhada.