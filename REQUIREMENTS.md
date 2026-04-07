
# Backend Engineer Challenge: "The Order Flow Firehose"

## 1. Summary & Objective

In modern trading systems, the ability to ingest, normalize, and store market data with **zero loss** and **minimal latency** is critical. Your task is to build a high-performance data pipeline that consumes real-time market data (order books and trades) from a major exchange and provides a clean, unified interface for downstream researchers.

### The Mission

Build a real-time ingestion engine and a companion Data SDK to retrieve clean, flattened market snapshots.

**Core Requirements:**

* **Ingestion:** Subscribe to streams for **TOP 20 crypto futures pairs** simultaneously (Reference: [Top Volume Crypto Futures](https://www.binance.com/en/markets/futures-perpetual?p=1)).
* **Normalization:** Maintain an in-memory L2 order book and merge it with trade events.
* **Deduplication:** Ensure idempotent storage; no duplicate events, even across WebSocket reconnections.
* **Storage:** Persistence in a database of your choice.
* **REST API:** Create a fast backend API in **Golang + Fiber v3**. Expose an `/sse` endpoint (Server-Sent Events) to stream the live normalized market data.
* **UI Dashboard:** Build a real-time web interface using **Vite + React + TypeScript** to consume the `/sse` feed and visualize the live stream.
* **SDK Interface:** Develop a library (`binance_sdk`) that exposes a `load_data()` function. While the underlying data mapping is up to you, the primary user interface must be a native Python package. You should also enable it to be used in Golang to stream and get the latest data.
---

## 2. Technical Stack

* **Streaming API:** Written using **Golang + Fiber v3**.
* **Ingestion**: Any language or framework.
* **Frontend Dashboard:** Built using **Vite**, **React**, and **TypeScript**.
* **Data SDK:** You can write it in any language, but the SDK interface must be exposed natively in **Python**.
* **SDK Interface:** Python.

---

## 3. Input Data & Connectivity

You will need to discover and interface with the [Binance WebSocket Streams](https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams). Please read the official documentation to obtain the correct stream URLs, channels, and data types for depth and aggregated trade events.

---

## 4. Data Mapping & Normalization

**Critical Logic:**
You must maintain a local data structure for the order book. When a trade event arrives, create a snapshot combining the trade data with the **current** top 6 levels of the book. 

| Target Field | Data Type      | Description                  |
| :----------- | :----------- | :--------------------------- |
| `timestamp`  | `datetime64[ns]` | Event time in milliseconds. |
| `price`      | `float64` | Latest executed trade price.                    |
| `qty`        | `float64` | Latest executed trade quantity.                 |
| `amount`     | `float64` | `price * qty`                                   |
| `bp1-bp6`    | `float64` | Top 6 bid prices (multiple columns).           | 
| `bq1-bq6`    | `float64` | Top 6 bid quantities (multiple columns).                           |
| `ap1-ap6`    | `float64` | Top 6 ask prices (multiple columns).            |
| `aq1-aq6`    | `float64`    | Top 6 ask quantities (multiple columns).                           |

---

## 5. SDK Specification and Schema

Your `binance_sdk` must abstract database complexity from the user (e.g., exposing an interface for historical backfills). Data shapes, specific schema structure, and implementation details are left to your design.
```python
from binance_sdk import load_data
import numpy as np
import pandas as pd
import datetime

DatetimeType = datetime.datetime | pd.Timestamp | np.datetime64 | str

# Your choice of how to implement type hints and returned data structures...
def load_data(symbol: str, start_time: DatetimeType, end_time: DatetimeType) -> pd.DataFrame:
    """
    Your implementation here
    """
    pass
```

---

## 6. Deliverables
- **Docker & Compose:** Provide `docker-compose.yml` configuring the required containers cleanly.
- **Backend API:** Written in Golang + Fiber v3.
- **Web Dashboard:** Vite + React + TypeScript website to show stream data where users can select symbols to see changes. Add authentication layer and a statistics dashboard.
- **Data SDK:** A packaged SDK (with Python interface) abstracting the database logic. Users can also use the SDK natively in Golang to stream or get the latest data.


## 7. Submission
### AI Policy

The use of AI is **strongly encouraged**. You must include an `AI.md` file in the root directory documenting:

1. Which tools were used
2. Which parts of the code were generated vs. manually written
3. How to setup project with AI tools or agents
4. How you prompted the AI to handle complex logic (e.g., order book synchronization)

### Project Requirements

* **README.md:** Include a `README.md` at the root of your repository covering:
  * A brief summary of your solution and architecture.
  * How to run the project locally (setup and `docker-compose` instructions).
  * Your choice of language/framework for the ingestion engine and backend API, and why.
  * Any third-party libraries used and their trade-offs.
* **Infrastructure:** Provide a `docker-compose.yml` that spins up the ingestor, backend API, frontend UI, and the database.
* **Backfill:** On startup, the engine should fetch the last 15 minutes of historical trades/depth via a REST API to ensure the database is not empty.
* **Evidence:** In the `/evidence` folder, include:

  * `logs.txt`: Showing successful connection and ingestion of 20+ pairs
  * `analysis.ipynb`: A notebook demonstrating `load_data()` results and a basic plot of the spread over time
  * Screenshots of your React dashboard displaying live data via the `/sse` route.
* **Submission:** Push your solution to a public GitHub repository.

---

## 8. Tips for Success

* **Memory Efficiency:** Keep the in-memory order book lean; avoid storing unnecessary levels.
* **Reliability:** Handle WebSocket disconnections with automatic exponential backoff.