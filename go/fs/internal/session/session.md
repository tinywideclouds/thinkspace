# Session Orchestration Layer

The `session` package serves as the central brain of the ThinkSpace application. It orchestrates the high-level human-AI interaction loop by connecting the LLM manager, the workspace domain, and the user interface.

## Core Components

* **`Manager`:** The top-level singleton injected into the API server. It is responsible for caching active `ChatSession` instances and providing a clean dependency-injected entrypoint for executing turns (`RunTurn`).
* **`ChatSession`:** Represents the active state of a single conversation thread. It manages physical metadata locks, flushes state to disk, builds the graph topology, and triggers the context assembly process before handing off to the `TurnCoordinator`.
* **`BundleAssembler`:** A discrete utility invoked by the `ChatSession` to walk the BadgerDB graph backwards, filter active sticky digests and space rules, map the physical directory tree, and format the persona template into a finalized `ThinkBundle`.
* **`TurnCoordinator`:** The struct responsible for managing the LLM execution stream during a single turn. It receives the assembled `ThinkBundle`, calls the LLM, intercepts tool calls (`query_lens`, `propose_change`), and routes sub-agent tasks to the `FlowEngine`.
* **`FlowEngine`:** The generic execution engine that runs configured sequences of sub-agent tasks, handling sandbox provisioning, user UI reviews (Delegation Strategies), and telemetry emissions.
* **`UserInterface`:** An interface abstracting the frontend. This allows the `TurnCoordinator` to operate identically whether it is communicating via a CLI (`TerminalUI`) or a Web Server (`WebSocketUI`).

## Delegation Strategies

When sub-agents propose code changes via a Fan-Out Flow, the user can select one of four strategies:
* **Skip (`StrategySkip`):** Automatically rejects all proposals and continues the chat.
* **Manual (`StrategyManual`):** The user manually previews and evaluates the candidate branches without AI assistance.
* **Review (`StrategyReview`):** The Manager LLM evaluates the candidate diffs, provides a comparative summary, and recommends an approach.
* **Refine (`StrategyRefine`):** The Manager LLM evaluates the candidates and immediately synthesizes an ultimate best version based on its evaluation.

## Turn Lifecycle

A single execution turn executed by the `TurnCoordinator` follows a recursive loop (bounded to a maximum of 3 iterations):

1. **Manager Generation:** The Manager LLM streams its response based on the assembled context, which is routed directly to the UI.
2. **Tool Interception:** The `TurnCoordinator` parses the model output for tool invocations.
    *   **Retrieval Augmentation (`query_lens`):** If the LLM requests more context about a specific semantic tag, the `TurnCoordinator` queries the BadgerDB graph for the raw events matching that tag, injects the subgraph into the context array, and recursively loops back to Step 1.
    *   **Sub-Agent Delegation (`propose_change`):** If the LLM is ready to act, it calls `propose_change`, breaking the loop and initiating the fan-out sequence.
3. **Event-Driven Fan-Out:** The `TurnCoordinator` triggers the `FlowEngine`. Sub-agents run concurrently in isolated virtual sandboxes, broadcasting structured lifecycle events in real-time.
4. **Verification & Strategy:** The CI verifier evaluates the code. The UI prompts the user to select a `DelegationStrategy`.
5. **Resolution:** The user previews the final candidates and explicitly accepts or rejects them.
6. **Checkpoint:** The `TurnCoordinator` commands the workspace service to permanently snapshot the thread state in the Git layer.


```python
import os

def find_files(filename, search_path):
    result = []
    for root, dir, files in os.walk(search_path):
        for file in files:
            if filename.lower() in file.lower():
                result.append(os.path.join(root, file))
    return result

print(find_files("readme", "go/fs"))
print(find_files("session", "go/fs"))


```

```text
[]
[]


```

```python
import os
print(os.listdir('go/fs/internal/session') if os.path.exists('go/fs/internal/session') else "Path not found")


```

```text
['tools', 'turn_executor.go', 'worker_engine.go', 'synthesize_engine.go', 'chat_engine.go', 'turn_executor_test.go']


```

```python
import glob
print(glob.glob('go/**/*.md', recursive=True))

```

## The Manager Subgraph: Fan-Out Diamond Topology

The `WorkerEngine` executes parallel agent workflows (fan-out) using a strict "Diamond" DAG topology. This ensures that multi-agent delegation remains mathematically unbroken, visually encapsulated, and capable of yielding control back to the user without fracturing timeline rehydration.

Instead of cluttering the Main Timeline with parallel execution nodes, the Manager's turn acts as a bridge to a dedicated Subgraph. The flow diverges into $N$ branches, verifies them independently, and joins them back together before yielding to the user.

### Execution Sequence & Edge Routing

**1. The Origin (Main Timeline)**
The flow is initiated by the Manager LLM. The Manager's ephemeral response node (`turnID`) acts as the structural head of the workflow.

* **Node:** `EventModel` (Manager's Turn)
* **ID:** `turnID` (Parent edge points to the User's `catalystID`)

**2. The Flow Card (Main Timeline)**
A visual anchor is placed on the Main Timeline to represent the execution of the subgraph.

* **Node:** `EventFlowLifecycle`
* **ID:** `flowID`
* **Edge:** `parent_id` $\rightarrow$ `turnID`

**3. The Subgraph Fork (Cross-Boundary)**
Inside the subgraph, the Manager spawns $N$ distinct agents based on the `propose_change` tool's `agent_instructions` array.

* **Node:** `EventPrompt` ($N$ distinct sub-agent prompts)
* **Edge:** `parent_id` $\rightarrow$ `turnID`
* *Note: This is the first cross-boundary edge. The subgraph roots itself directly into the Manager's turn on the Main Timeline.*

**4. Parallel Execution & Resolution (Subgraph)**
Each agent executes independently within isolated sandboxes.

* **Node:** `EventModel` (Agent generation, points to its respective `EventPrompt`)
* **Node:** `EventCandidateResolution` (Verification pass/fail, points to the `EventModel`)

**5. The Subgraph Join**
Once all $N$ branches complete, the graph converges to a single state summarizing the results.

* **Node:** `EventFlowLifecycle` (The Join Node)
* **ID:** `joinNodeID`
* **Edge:** `parent_0`...`parent_N` $\rightarrow$ Every leaf `EventCandidateResolution` ID from step 4.

**6. Yield to Main Timeline (Cross-Boundary)**
The workflow pauses and yields control back to the user on the Main Timeline for manual intervention or strategy selection.

* **Node:** `EventSystem` (Strategy Options Node)
* **ID:** `optionsNodeID`
* **Edge:** `parent_id` $\rightarrow$ `joinNodeID`
* *Note: This weaves the tail of the Subgraph back into the Main Timeline.*

**7. Continuation: The Synthesis Return**
If the user selects a continuation strategy like **Refine** or **Synthesize**, the `SynthesizeEngine` steps in. The flow drops *back* into the subgraph to encapsulate the synthesis, preventing Main Timeline clutter.

* **Node:** `EventPrompt` (The Synthesis Prompt)
* **Edge:** `parent_id` $\rightarrow$ `optionsNodeID`
* *Note: Linking to the Options Node brings the entire multi-branch context forward.*

### Architectural Guarantees

* **No Dangling Edges:** Because all subgraph prompts hook back to the persisted `turnID`, and the Options node hooks to the `joinNodeID`, the DAG remains unbroken during BadgerDB syncs and context rehydration.
* **Context Compression:** Main Timeline rehydration only pulls the Flow Card and the Options Node, neatly bypassing the massive token weight of the internal subgraph attempts unless explicitly traversed.