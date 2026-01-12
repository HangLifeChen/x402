# Getting Started

The zkStash REST API provides a powerful interface for interacting with the memory layer directly. This guide covers the essential concepts of authentication and payments required to use the API.

## Base URL

All API requests should be made to:

```
https://api.zkstash.ai
```

## Authentication

zkStash uses a wallet-based authentication mechanism. Every request must be signed by a valid EVM or Solana wallet.

### Required Headers

Include the following headers in every request:

| Header | Description |
| :--- | :--- |
| `x-wallet-address` | Your public wallet address (e.g., `0x...` or `SolanaAddress...`). |
| `x-wallet-signature` | A cryptographic signature of the request details. |
| `x-wallet-timestamp` | The current Unix timestamp (in milliseconds). Must be within 2 minutes of the server time. |

### Generating the Signature

To generate the `x-wallet-signature`, you must sign a **canonical message** constructed from the request details.

#### 1. Construct the Canonical Message

The message format is:
```
METHOD|PATH|BODY_HASH|TIMESTAMP
```

- **METHOD**: The HTTP method in uppercase (e.g., `POST`, `GET`, `PATCH`).
- **PATH**: The request path (e.g., `/memories`). Do not include query parameters.
- **BODY_HASH**: The SHA-256 hash of the request body (hex string).
    - If there is no body (e.g., GET requests), use the hash of an empty string.
- **TIMESTAMP**: The exact timestamp value used in the `x-wallet-timestamp` header.

#### 2. Sign the Message

Sign the canonical message string using your wallet's private key.
- **EVM**: Use `signMessage()`.
- **Solana**: Use `signMessages()`.

#### Example (Node.js)

```javascript
const crypto = require('crypto');
const ethers = require('ethers'); // For EVM
const nacl = require('tweetnacl'); // For Solana
const bs58 = require('bs58');

async function generateHeaders(method, path, body, privateKey, chain = 'evm') {
  const timestamp = Date.now().toString();
  
  // 1. Hash the body
  const bodyString = body ? JSON.stringify(body) : '';
  const bodyHash = crypto.createHash('sha256').update(bodyString).digest('hex');
  
  // 2. Create Canonical Message
  const message = `${method.toUpperCase()}|${path}|${bodyHash}|${timestamp}`;
  
  let signature;
  let address;

  if (chain === 'evm') {
    const wallet = new ethers.Wallet(privateKey);
    address = wallet.address;
    signature = await wallet.signMessage(message);
  } else {
    // Solana implementation
    const keyPair = nacl.sign.keyPair.fromSecretKey(bs58.decode(privateKey));
    address = bs58.encode(keyPair.publicKey);
    const messageBytes = new TextEncoder().encode(message);
    const signatureBytes = nacl.sign.detached(messageBytes, keyPair.secretKey);
    signature = Buffer.from(signatureBytes).toString('base64');
  }

  return {
    'x-wallet-address': address,
    'x-wallet-timestamp': timestamp,
    'x-wallet-signature': signature,
    'Content-Type': 'application/json'
  };
}
```

## Payment Flow (x402)

zkStash implements the **x402** protocol for metered API usage. This allows for permissionless, pay-as-you-go access using crypto.

### 402 Payment Required

If your account (wallet) does not have enough free credits, the API will return a `402 Payment Required` status. The response body will contain the payment requirements.

**Example 402 Response:**

```json
{
  "x402Version": 1,
  "error": "X-PAYMENT header is required",
  "accepts": [
    {
      "network": "solana-devnet",
      "token": "USDC",
      "amount": "0.1",
      "recipient": "CKPKJWNdJEqa81x7CkZ14BVPiY6y16Sxs7owznqtWYp5"
    },
    {
      "network": "base-sepolia",
      "token": "USDC",
      "amount": "0.1",
      "recipient": "0x..."
    }
  ]
}
```

### Handling Payments

1. **Check for 402**: Inspect the response status code.
2. **Select Network**: Choose a supported network from the `accepts` list (e.g., `solana-devnet`).
3. **Execute Transaction**: Send the specified `amount` of `token` to the `recipient` address on-chain.
4. **Retry with Proof**: Retry the original API request, adding the `x-payment` header.

The `x-payment` header should contain a base64-encoded JSON object with the transaction details (proof).

**Supported Networks:**
- `solana-devnet`
- `base-sepolia`

## Response Codes

| Status | Meaning | Description |
| :--- | :--- | :--- |
| `200` | OK | The request was successful. |
| `400` | Bad Request | Invalid parameters or body schema. Check the error message for details. |
| `401` | Unauthorized | Invalid signature, timestamp expired (> 2 mins), or malformed headers. |
| `402` | Payment Required | Insufficient credits. Payment required to proceed. |
| `404` | Not Found | The requested resource (memory, schema) does not exist. |
| `500` | Internal Error | Something went wrong on the server. |


# Memories Endpoints

Memories are the core data units in zkstash. These endpoints allow you to store, retrieve, and manage the long-term memory of your agents.

## Create Memory

This endpoint supports two modes:
1. **Extraction Mode**: Pass a `conversation` array to let zkstash's AI extract memories automatically.
2. **Direct Mode**: Pass a `memories` array to store structured data directly without LLM processing.

**Endpoint:** `POST /memories`

### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `agentId` | `string` | Yes | Unique identifier for the agent. |
| `subjectId` | `string` | No | Subject ID for multi-tenant isolation. |
| `conversation` | `array` | No* | List of message objects for AI extraction. |
| `memories` | `array` | No* | List of structured memories to store directly. |
| `threadId` | `string` | No | Identifier for the conversation thread. |
| `schemas` | `string[]` | No | List of schema names to use for extraction. |
| `ttl` | `string` | No | Default TTL for all memories (e.g., `"1h"`, `"24h"`, `"7d"`). |
| `expiresAt` | `number` | No | Default expiry timestamp (ms) for all memories. |

> **Note:** Either `conversation` or `memories` must be provided. Incremental extraction is automatic—if you provide message `id`s, the server will skip any messages it has already processed.

**Conversation Object:**
```json
{
  "id": "msg_123", // Recommended for automatic incremental extraction
  "role": "user" | "assistant" | "system",
  "content": "string"
}
```

**Direct Memory Object:**
```json
{
  "kind": "UserProfile", // Must match a registered schema
  "data": { "name": "Alice", "age": 30 },
  "id": "mem_123", // Optional (see below)
  "ttl": "7d", // Optional: memory-specific TTL
  "expiresAt": 1735689600000 // Optional: explicit expiry timestamp (ms)
}
```

> **TTL Priority:** Memory-level `ttl`/`expiresAt` overrides request-level defaults. Memories without TTL are permanent.

> **ID Behavior:**
> - **Schemas with `uniqueOn`** (e.g., `UserProfile`): ID is auto-generated. New memories automatically supersede existing ones matching the unique fields.
> - **Schemas without `uniqueOn`** (e.g., `Interaction`): Omit `id` to create, include `id` to update an existing record.

### Example: Extraction Mode

```json
{
  "agentId": "agent-007",
  "conversation": [
    { "role": "user", "content": "My favorite color is blue." },
    { "role": "assistant", "content": "Noted." }
  ]
}
```

### Example: Direct Mode

```json
{
  "agentId": "agent-007",
  "memories": [
    { "kind": "UserProfile", "data": { "favoriteColor": "blue" } }
  ]
}
```

### Example: With TTL

```json
{
  "agentId": "agent-007",
  "ttl": "24h",
  "memories": [
    { "kind": "SessionContext", "data": { "task": "booking" } },
    { "kind": "Reminder", "data": { "text": "Follow up" }, "ttl": "1h" }
  ]
}
```

### Response

Returns the created and updated memories.

```json
{
  "success": true,
  "created": [
    {
      "kind": "UserProfile",
      "metadata": { "favoriteColor": "blue" }
    }
  ],
  "updated": []
}
```

---

## Search Memories

Search for memories using semantic similarity and metadata filters. Supports searching your own memories and shared memories from other agents via grants.

**Endpoint:** `GET /memories/search`

### Query Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `query` | `string` | Yes | The natural language query to search for (e.g., "user preferences"). |
| `agentId` | `string` | No | Filter by agent ID. |
| `subjectId` | `string` | No | Filter by subject ID (tenant). |
| `threadId` | `string` | No | Filter by specific thread ID. |
| `kind` | `string` | No | Filter by memory schema type (e.g., `UserProfile`). |
| `tags` | `string` | No | Comma-separated list of tags to filter by. |
| `limit` | `number` | No | Max number of results (default: 10). |
| `mode` | `string` | No | Response format: `llm` (default), `answer` (QA), or `map` (Graph). |
| `scope` | `string` | No | Search scope: `own`, `shared`, or `all` (default). See [Memory Sharing](/core-concepts/sharing). |

### Response Modes

| Mode | Description |
| :--- | :--- |
| `llm` | **Default.** Returns semantically structured memories optimized for LLM consumption. |
| `answer` | Returns a concise, grounded answer to the query. |
| `map` | Returns a memory map with topical clusters. |

### Headers

| Header | Type | Description |
| :--- | :--- | :--- |
| `X-Grants` | `string` | Base64-encoded JSON array of signed grants for accessing shared memories. |

### Search Scope

The `scope` parameter controls which namespaces are searched:

| Scope | Searches Own | Searches Shared | Notes |
| :--- | :--- | :--- | :--- |
| `own` | ✅ | ❌ | Only your memories, grants are ignored |
| `shared` | ❌ | ✅ | Only granted namespaces (requires grants) |
| `all` | ✅ | ✅ | Both own and granted (default) |

### Using Grants

To search shared memories, include grants in the `X-Grants` header:

```bash
# Create base64-encoded grants
GRANTS=$(echo '[{"p":{"f":"0xGrantor...","g":"0xYourAddress...","e":1735689600},"s":"0x...","c":"evm"}]' | base64)

# Search with grants
curl "https://api.zkstash.ai/v1/memories/search?query=findings&agentId=researcher" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Grants: $GRANTS"
```

### Example Request

```
GET /memories/search?query=What+does+he+like?&agentId=agent-007&limit=5
```

### Response (LLM Mode - Default)

The default `llm` mode returns semantically structured memories optimized for LLM consumption:

```json
{
  "success": true,
  "memories": [
    {
      "id": "mem_123",
      "kind": "UserProfile",
      "quality": {
        "relevance": 0.89,
        "confidence": 0.95
      },
      "data": {
        "favoriteColor": "blue",
        "theme": "dark"
      },
      "context": {
        "when": "2024-01-15T10:30:00Z",
        "mentions": [
          { "name": "User", "type": "person" }
        ],
        "tags": ["preferences"],
        "isLatest": true
      },
      "source": "own"
    },
    {
      "id": "mem_456",
      "kind": "ResearchFinding",
      "quality": {
        "relevance": 0.85,
        "confidence": 0.90
      },
      "data": {
        "topic": "AI agents",
        "summary": "Key findings about agent memory..."
      },
      "context": {
        "mentions": [
          { "name": "AI agents", "type": "concept" }
        ],
        "tags": ["research"],
        "isLatest": true
      },
      "source": "shared:researcher"
    }
  ],
  "searchedAt": "2024-01-20T14:30:00Z"
}
```

#### Response Fields

| Field | Description |
| :--- | :--- |
| `id` | Memory identifier for updates/references. |
| `kind` | Schema type (e.g., `UserProfile`, `temporal_event`). |
| `quality.relevance` | Search relevance score (0-1). |
| `quality.confidence` | Extraction certainty (0-1). |
| `data` | User-defined schema fields (the actual content). |
| `context.when` | ISO 8601 event timestamp (if temporal). |
| `context.mentions` | Typed entity references for disambiguation. |
| `context.tags` | Topical tags. |
| `context.isLatest` | `true` if not superseded by newer memory. |
| `source` | Provenance: `own`, `shared`, or `shared:{agentId}`. |
| `searchedAt` | Query timestamp for relative temporal reasoning. |

---

## Get Memory

Retrieve a single memory by its ID.

**Endpoint:** `GET /memories/[id]`

### Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `id` | `string` | Yes | The unique ID of the memory. |

### Response

```json
{
  "success": true,
  "memory": {
    "id": "mem_123...",
    "kind": "UserProfile",
    "data": { ... }
  }
}
```

---

## Update Memory

Update a memory's metadata or expiration.

**Endpoint:** `PATCH /memories/[id]`

### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `tags` | `string[]` | No | New list of tags for the memory. |
| `expiresAt` | `number \| null` | No | Set expiry timestamp (ms), or `null` to remove expiry and make permanent. |

### Example: Update Tags

```json
{
  "tags": ["important", "verified"]
}
```

### Example: Set Expiry

```json
{
  "expiresAt": 1735689600000
}
```

### Example: Remove Expiry (Make Permanent)

```json
{
  "expiresAt": null
}
```

### Response

```json
{
  "success": true,
  "memory": { ... }
}
```

---

## Delete Memory

Delete a single memory by ID.

**Endpoint:** `DELETE /memories/[id]`

### Response

```json
{
  "success": true
}
```

---

## Batch Operations

For improved performance when working with multiple memories, use batch endpoints. Batch operations use **flat pricing** (2× single operation) regardless of batch size, offering significant savings at scale.

| Endpoint | Cost | Max Items | vs Individual |
| :--- | :--- | :--- | :--- |
| `batch/search` | 4 CC | 10 queries | Up to 80% savings |
| `batch/delete` | 2 CC | 100 IDs | Up to 98% savings |
| `batch/update` | 4 CC | 100 IDs | Up to 98% savings |

### Batch Search

Search multiple queries in parallel.

**Endpoint:** `POST /memories/batch/search`

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `queries` | `array` | Yes | Array of search queries (max 10). |
| `scope` | `string` | No | Search scope: `own`, `shared`, or `all` (default). |

```json
{
  "queries": [
    { "query": "user preferences", "filters": { "agentId": "assistant" } },
    { "query": "recent tasks", "filters": { "agentId": "assistant" } }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "results": [
    { "memories": [...] },
    { "memories": [...] }
  ]
}
```

### Batch Delete

Delete multiple memories by ID.

**Endpoint:** `POST /memories/batch/delete`

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `ids` | `string[]` | Yes | Array of memory IDs to delete (max 100). |

```json
{
  "ids": ["mem_abc123", "mem_def456"]
}
```

**Response:**
```json
{
  "success": true,
  "deleted": 2
}
```

### Batch Update

Update multiple memories with the same changes.

**Endpoint:** `POST /memories/batch/update`

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `ids` | `string[]` | Yes | Array of memory IDs to update (max 100). |
| `update` | `object` | Yes | Update payload (same as single update). |

```json
{
  "ids": ["mem_abc123", "mem_def456"],
  "update": {
    "tags": ["archived"],
    "expiresAt": 1735689600000
  }
}
```

**Response:**
```json
{
  "success": true,
  "updated": 2
}
```

# Schemas Endpoints

Schemas define the structure of the memories your agents can create. They act as blueprints for the extraction process.

## Register Schema

Register a new schema for memory extraction.

**Endpoint:** `POST /schemas`

### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | `string` | Yes | Unique name for the schema (e.g., `UserProfile`). |
| `description` | `string` | Yes | Description of what this schema represents (used by the LLM). |
| `uniqueOn` | `string[]` | No | Fields that identify the same entity for auto-supersede. |
| `schema` | `string` | Yes | JSON Schema string defining the data structure. |

### uniqueOn Behavior

- `["kind"]`: One memory per schema type (e.g., user profile). New memories supersede previous.
- `["email"]`: One memory per unique email address.
- Omit: Multiple instances allowed (e.g., tasks, events).

### Example Request

```json
{
  "name": "Task",
  "description": "A task that the user needs to complete",
  "schema": "{\"type\":\"object\",\"properties\":{\"title\":{\"type\":\"string\"},\"status\":{\"type\":\"string\"}}}"
}
```

### Example with Auto-Supersede

```json
{
  "name": "UserProfile",
  "description": "User preferences and settings",
  "uniqueOn": ["kind"],
  "schema": "{\"type\":\"object\",\"properties\":{\"theme\":{\"type\":\"string\"},\"language\":{\"type\":\"string\"}}}"
}
```

### Response

```json
{
  "success": true,
  "schema": {
    "name": "Task",
    "description": "..."
  }
}
```

---

## List Schemas

Retrieve all schemas registered by your account.

**Endpoint:** `GET /schemas`

### Response

```json
{
  "success": true,
  "schemas": [
    { "name": "UserProfile", ... },
    { "name": "Task", ... }
  ]
}
```

---

## Get Schema

Get details of a specific schema.

**Endpoint:** `GET /schemas/[name]`

### Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | `string` | Yes | The name of the schema. |

### Response

```json
{
  "success": true,
  "schema": { ... }
}
```

---

## Update Schema

Update an existing schema's description or structure.

**Endpoint:** `PATCH /schemas/[name]`

### Request Body

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `description` | `string` | No | New description. |
| `schema` | `string` | No | New JSON Schema string. |
| `uniqueOn` | `string[]` | No | New unique fields for auto-supersede. |

### Example Request

```json
{
  "description": "Updated task definition with priority"
}
```

---

## Delete Schema

Delete a schema. This prevents future memories from being created with this schema, but does not delete existing memories.

**Endpoint:** `DELETE /schemas/[name]`

### Response

```json
{
  "success": true,
  "deleted": 1
}
```

# Credits Endpoints

Manage your credit balance for API operations and storage insurance.

## Deposit Credits

Deposit credits into your account via x402 payment. This enables autonomous agents to fund their own storage insurance without human intervention.

**Endpoint:** `POST /credits/deposit`

### Query Parameters

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `amount_usd` | `number` | No | Amount to deposit in USD. Min: $0.10, Max: $1000. Defaults to $0.10 if not specified. |

### x402 Payment Flow

This endpoint uses the **x402 protocol** for permissionless payments:

1. **Initial Request**: Call the endpoint without payment
2. **402 Response**: Receive payment requirements (supported networks, amounts, recipient)
3. **Sign Payment**: Use your wallet to sign an x402 payment
4. **Retry with Payment**: Retry the request with `x-payment` header containing the signed payment
5. **Success**: Receive confirmation and updated balance

### Example: Initial Request

```bash
curl -X POST "https://api.zkstash.ai/v1/credits/deposit?amount_usd=5" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### Example: 402 Response

```json
{
  "x402Version": 1,
  "error": "Payment required for credit deposit",
  "deposit": {
    "amount_usd": 5,
    "credits": 5000,
    "description": "Credits will be added to your balance for future operations and storage insurance."
  },
  "accepts": [
    {
      "scheme": "exact",
      "network": "solana",
      "maxAmountRequired": "5000000",
      "resource": "https://api.zkstash.ai/v1/credits/deposit",
      "description": "Deposit 5,000 credits ($5.00 USD)",
      "mimeType": "application/json",
      "payTo": "...",
      "extra": { "feePayer": "..." }
    },
    {
      "scheme": "exact", 
      "network": "base",
      "maxAmountRequired": "5000000",
      "resource": "https://api.zkstash.ai/v1/credits/deposit",
      "description": "Deposit 5,000 credits ($5.00 USD)",
      "mimeType": "application/json",
      "payTo": "..."
    }
  ]
}
```

### Example: Success Response

After providing a valid `x-payment` header:

```json
{
  "success": true,
  "deposit": {
    "amount_usd": 5,
    "credits": 5000
  },
  "balance": {
    "credits": 5000,
    "usd": 5
  },
  "message": "Deposit successful. Storage insurance is now active."
}
```

---

## Use Cases

### Storage Insurance for Free Plan

Free plan memories are deleted after 7 days by default. To prevent deletion:

1. Deposit credits using this endpoint
2. The daily cron job will deduct ~3.33 credits per 1,000 memories to "insure" your storage
3. As long as you maintain a positive balance, your memories persist

### Autonomous Agent Self-Funding

Agents that earn revenue (e.g., by providing services) can use those funds to:

1. Pay for their own storage costs
2. Pre-fund future API operations
3. Operate indefinitely without human intervention

This enables **self-sovereign AI agents**—a capability unique to crypto-native platforms.

---

## Supported Networks

| Network | Token | Environment |
| :--- | :--- | :--- |
| Solana | USDC | Production |
| Base | USDC | Production |
| Solana Devnet | USDC | Development |
| Base Sepolia | USDC | Development |


# Attestations Endpoints

Attestations let agents prove claims about their memories to other agents without revealing the actual content. zkStash signs claims with Ed25519, enabling local verification without API calls.

For conceptual overview and use cases, see [Attestations Concepts](/core-concepts/attestations).

## Create Attestation

Create a signed attestation about your memories. The attestation can be shared with other agents to prove claims without revealing actual memory content.

**Endpoint:** `POST /attestations`

**Cost:** 3 CC

### Request Body

| Field        | Type     | Required    | Description                                          |
| :----------- | :------- | :---------- | :--------------------------------------------------- |
| `claim`      | `string` | Yes         | The type of claim to attest.                         |
| `query`      | `string` | Conditional | Search query (required for `has_memories_matching`). |
| `filters`    | `object` | No          | Filters for the search (agentId, kind, tags).        |
| `threshold`  | `number` | Conditional | Minimum count (required for `memory_count_gte`).     |
| `schemaName` | `string` | Conditional | Schema name (required for `has_schema`).             |
| `expiresIn`  | `string` | No          | Attestation validity duration (default: `"24h"`).    |

### Claim Types

| Claim                   | Description                         | Required Params |
| :---------------------- | :---------------------------------- | :-------------- |
| `has_memories_matching` | Agent has memories matching a query | `query`         |
| `memory_count_gte`      | Agent has ≥ N memories              | `threshold`     |
| `has_schema`            | Agent has a registered schema       | `schemaName`    |

### Example: Prove You Have Matching Memories

```json
{
  "claim": "has_memories_matching",
  "query": "cooking recipes",
  "filters": {
    "agentId": "recipe-bot",
    "kind": "Recipe"
  },
  "expiresIn": "24h"
}
```

### Example: Prove Memory Count

```json
{
  "claim": "memory_count_gte",
  "threshold": 10,
  "filters": {
    "kind": "Interaction"
  }
}
```

### Example: Prove Schema Exists

```json
{
  "claim": "has_schema",
  "schemaName": "UserProfile"
}
```

### Response

```json
{
  "success": true,
  "attestation": {
    "claim": "has_memories_matching",
    "params": {
      "query": "cooking recipes",
      "filters": { "agentId": "recipe-bot", "kind": "Recipe" }
    },
    "result": {
      "satisfied": true,
      "matchCount": 15,
      "namespace": "0x7f3a9c2b..."
    },
    "issuedAt": 1703123456,
    "expiresAt": 1703209856,
    "issuer": "zkstash.ai"
  },
  "signature": "0x...",
  "publicKey": "0x...",
  "algorithm": "Ed25519"
}
```

---

## Verifying Attestations

Attestations use **Ed25519 signatures** and are verified locally—no API call required.

### 1. Get the Public Key

Fetch the public key once and cache it:

```bash
curl https://api.zkstash.ai/.well-known/zkstash-keys.json
```

```json
{
  "attestationPublicKey": "0x...",
  "algorithm": "Ed25519",
  "issuedAt": 1703123456789
}
```

### 2. Serialize the Attestation

Create a canonical JSON string with **sorted keys** (deterministic serialization):

```json
{
  "claim": "has_memories_matching",
  "expiresAt": 1703209856,
  "issuedAt": 1703123456,
  "issuer": "zkstash.ai",
  "params": {
    "filters": { "agentId": "recipe-bot", "kind": "Recipe" },
    "query": "cooking recipes"
  },
  "result": {
    "matchCount": 15,
    "namespace": "0x7f3a9c2b...",
    "satisfied": true
  }
}
```

### 3. Verify the Signature

Use any Ed25519 library to verify:

**Python:**

```python
from nacl.signing import VerifyKey
import json

def verify_attestation(attestation: dict, signature: str, public_key: str) -> bool:
    # Canonical JSON (sorted keys)
    message = json.dumps(attestation, sort_keys=True, separators=(',', ':'))

    # Remove 0x prefix and convert
    sig_bytes = bytes.fromhex(signature[2:])
    key_bytes = bytes.fromhex(public_key[2:])

    verify_key = VerifyKey(key_bytes)
    try:
        verify_key.verify(message.encode(), sig_bytes)
        return True
    except:
        return False
```

**Node.js (without SDK):**

```javascript
import { verify } from "@noble/ed25519";

function verifyAttestation(attestation, signature, publicKey) {
  // Canonical JSON (sorted keys)
  const message = JSON.stringify(attestation, Object.keys(attestation).sort());

  const sigBytes = Buffer.from(signature.slice(2), "hex");
  const keyBytes = Buffer.from(publicKey.slice(2), "hex");

  return verify(sigBytes, Buffer.from(message), keyBytes);
}
```

### 4. Check Expiration

Always verify `expiresAt > currentTime` before trusting the attestation.

> **TypeScript SDK:** Use `client.verifyAttestation()` for a simpler API. See [SDK Attestations](/sdk-typescript/attestations).

---

## Source Attestations in Search

When searching shared memories via grants, the response includes source attestations that prove provenance:

```json
{
  "success": true,
  "memories": [
    {
      "id": "mem_123",
      "kind": "ResearchFinding",
      "metadata": {
        "topic": "AI agents",
        "contentHash": "0x7f3a9c2b..."
      },
      "source": "shared",
      "grantor": "0xAgentA..."
    }
  ],
  "sourceAttestations": {
    "0xAgentA...": {
      "attestation": {
        "claim": "shared_memories_from",
        "params": {
          "grantor": "0xAgentA...",
          "memoryHashes": ["0x7f3a9c2b..."]
        },
        "result": {
          "satisfied": true,
          "matchCount": 1,
          "namespace": "0x..."
        },
        "issuedAt": 1703123456,
        "expiresAt": 1703209856,
        "issuer": "zkstash.ai"
      },
      "signature": "0x..."
    }
  }
}
```

The source attestation proves:

- Memories came from the claimed grantor's namespace
- zkStash vouches for the provenance
- Memory hashes match what was returned