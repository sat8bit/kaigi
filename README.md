# Kaigi: AI Conversation Simulator

A multi-agent conversation simulator built in Go, where multiple AI agents with unique personas autonomously engage in conversation.

## What is this?

This project is a command-line tool that simulates a conversation among multiple AI agents, each with a distinct personality defined in a `personas.yaml` file. The agents' interactions are powered by Google's Gemini models. The entire conversation is then saved as a blog-post-ready Markdown file.

## Features

- **Persona-Driven Conversations:** AI agents' personalities, speaking styles, and backgrounds are defined in an external YAML file, making it easy to add or modify characters.
- **Autonomous Interaction:** Agents listen to each other and decide when to speak based on their personality, creating a natural, emergent dialogue.
- **Clean, Component-Based Architecture:** Built with loosely coupled components (`Bus`, `TurnManager`, `Supervisor`, `Renderer`), making the system easy to maintain and extend.
- **Automatic Shutdown:** The simulation automatically ends after a specified number of turns, preventing infinite loops and managing costs.
- **Markdown Output:** Each conversation is automatically saved as a well-formatted Markdown file, including Hugo-compatible front matter with the topic and participants as tags.
- **Portable:** Uses `go:embed` to bundle the persona definitions into a single executable binary, requiring no external dependencies at runtime.

## Getting Started

### Prerequisites

- Go 1.18 or later
- A Google Cloud Project with the Vertex AI API enabled.
- The following environment variables must be set:
  ```sh
  export PROJECT_ID="your-gcp-project-id"
  export LOCATION="your-gcp-region" # e.g., us-central1
  ```

### Running the Simulator

You can run the simulation using `go run`. Use flags to customize the conversation:

```sh
# Run with default settings (3 participants, 20 turns, topic: "今日の天気について")
go run .

# Run with a custom topic and number of turns
go run . -topic="AIに自我は宿るのか？" -turns=15

# Run with 5 participants
go run . -chas=5
```

### Available Flags

- `-topic`: Sets the initial topic for the conversation. (Default: "今日の天気について")
- `-turns`: Sets the maximum number of conversational turns before the simulation automatically shuts down. (Default: 20)
- `-chas`: Sets the number of AI agents participating in the conversation. (Default: 3)

### Output

The conversation log will be printed to the console in real-time. Upon completion, a Markdown file will be saved in the `./content/posts/` directory.

## Architecture Overview

The simulator is designed with a clear separation of concerns, orchestrated by several key components:

- **`Persona`**: Defines the personality and attributes of each AI agent. Loaded from `personas.yaml`.
- **`Cha`**: The "actor" agent that embodies a `Persona`. It listens to the conversation and uses the LLM to generate responses.
- **`Bus`**: A central message bus that broadcasts messages from each `Cha` to all other participants.
- **`TurnManager`**: A mutex-based manager that ensures only one `Cha` can "speak" at a time, preventing chaos.
- **`Supervisor`**: Monitors the conversation and gracefully shuts down the application when the maximum number of turns is reached.
- **`Renderer`**: A component responsible for output.
  - `ConsoleRenderer`: Renders the live conversation to the console.
  - `MarkdownRenderer`: Renders the complete conversation log into a formatted Markdown file upon shutdown.
