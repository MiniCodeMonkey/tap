---
title: Mermaid Diagrams
---

# Mermaid Diagrams

Tap has built-in support for [Mermaid](https://mermaid.js.org/) diagrams, allowing you to create flowcharts, sequence diagrams, and more using simple text syntax directly in your slides.

## Basic Usage

Add a mermaid code block to your slide:

````markdown
```mermaid
flowchart LR
    A[Start] --> B{Decision}
    B -->|Yes| C[Success]
    B -->|No| D[Retry]
```
````

The diagram renders automatically when your slide is displayed.

## Supported Diagram Types

Tap supports all diagram types that Mermaid provides:

| Type | Use Case |
|------|----------|
| Flowchart | Process flows, decision trees |
| Sequence | API calls, system interactions |
| Class | Object-oriented designs |
| State | State machines, workflows |
| Entity Relationship | Database schemas |
| Gantt | Project timelines |
| Pie | Data distribution |
| Git | Branch visualization |
| Mindmap | Concept mapping |
| Timeline | Historical events |

## Examples

### Flowchart

````markdown
```mermaid
flowchart TD
    A[Start] --> B{Is it working?}
    B -->|Yes| C[Great!]
    B -->|No| D[Debug]
    D --> B
```
````

### Sequence Diagram

````markdown
```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Database

    Client->>API: POST /users
    API->>Database: INSERT user
    Database-->>API: Success
    API-->>Client: 201 Created
```
````

### Class Diagram

````markdown
```mermaid
classDiagram
    class User {
        +String name
        +String email
        +login()
        +logout()
    }
    class Admin {
        +manageUsers()
    }
    User <|-- Admin
```
````

### State Diagram

````markdown
```mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> Review
    Review --> Published
    Review --> Draft
    Published --> [*]
```
````

### Entity Relationship

````markdown
```mermaid
erDiagram
    USER ||--o{ ORDER : places
    ORDER ||--|{ LINE_ITEM : contains
    PRODUCT ||--o{ LINE_ITEM : "ordered in"
```
````

## Theme Integration

Mermaid diagrams automatically adapt to your presentation theme. Each of Tap's 21 themes defines its own Mermaid color scheme (node fill, stroke, and text colors that match the theme's palette, plus a matching curve style), so a flowchart in `terminal` reads as terminal-green and the same diagram in `keynote` reads as a dark launch-stage palette.

When you change your presentation theme, all Mermaid diagrams re-render with matching colors. Diagrams render after the theme's fonts are ready, so labels are never measured against a fallback font.

### The `quiet` class

Most themes also define a quiet style for background nodes, such as the app servers behind the thing you are actually talking about. Mark those nodes `quiet` and they pick up each theme's own treatment:

````markdown
```mermaid
flowchart LR
    A[Load balancer] --> B[app-1]
    A --> C[app-2]
    class B,C quiet
```
````

Tap appends the theme's `classDef quiet` for you, unless the diagram defines its own `quiet` class or the theme sets no quiet style.

## Error Handling

If a diagram has a syntax error, Tap displays:
- An error message explaining the issue
- The original Mermaid code for debugging

Fix the syntax in your markdown file and the diagram will re-render on save.

## Tips

### Keep Diagrams Simple

Presentation diagrams should be readable from a distance. Favor:
- Fewer nodes (5-10 max per diagram)
- Short labels
- Clear directional flow

### Use Fragments for Reveals

Wrap diagrams in fragment syntax to reveal them with animation:

````markdown
<!-- fragment -->
```mermaid
flowchart LR
    A --> B --> C
```
````

### Combine with Layouts

Use the `two-column` layout to show a diagram alongside explanatory text:

````markdown
<!--
layout: two-column
-->

# Architecture

Our system uses three main components.

::right

```mermaid
flowchart TD
    API --> Cache
    Cache --> DB
```
````

## Quick Reference

| Feature | Syntax |
|---------|--------|
| Basic diagram | ` ```mermaid ... ``` ` |
| Flowchart (LR) | `flowchart LR` |
| Flowchart (TD) | `flowchart TD` |
| Sequence | `sequenceDiagram` |
| Class | `classDiagram` |
| State | `stateDiagram-v2` |
| ER Diagram | `erDiagram` |

## Next Steps

- See [Mermaid documentation](https://mermaid.js.org/syntax/flowchart.html) for full syntax reference
- Learn about [Layouts](/guide/layouts) for positioning diagrams
- Explore [Animations](/guide/animations-transitions) for diagram reveals
