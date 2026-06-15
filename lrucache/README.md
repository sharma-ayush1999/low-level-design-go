# LRU Cache — Low Level Design

## Problem Statement

Design a thread-safe Least Recently Used (LRU) cache with a fixed capacity. When the cache is full and a new key is inserted, the key that was accessed least recently must be evicted to make room. All operations — `Get` and `Put` — must run in O(1) time.

---

## Requirements

1. The cache has a fixed capacity set at creation time.
2. `Get(key)` returns the value for the key if it exists, and marks that key as most recently used.
3. `Put(key, value)` inserts or updates the key-value pair and marks it as most recently used.
4. When capacity is exceeded on a `Put`, the least recently used key is evicted.
5. All operations are thread-safe — multiple goroutines may call `Get` and `Put` concurrently.
6. The cache supports any comparable key type and any value type (generics).

---

## Project Structure

```
lrucache/
├── lru_cache.go       # Core data structure: Node, LRUCache, all operations
└── lru_cache_demo.go  # Entry point / demo
```

---

## Design Patterns Used

| Pattern | Where | Why |
|---|---|---|
| **Doubly Linked List + Hash Map** | `LRUCache` internals | The canonical LRU data structure. The map gives O(1) key lookup; the list maintains insertion/access order. Together they make Get and Put both O(1). |
| **Sentinel Nodes** | `head`, `tail` in `LRUCache` | Dummy head and tail nodes eliminate all nil checks in list manipulation. Every real node always has a valid `prev` and `next`. |
| **Generics** | `Node[K, V]`, `LRUCache[K, V]` | The cache works with any key and value type without type assertions or `interface{}`. The compiler enforces type safety. |
| **Mutex** | `sync.RWMutex` in `LRUCache` | Protects the map and list from concurrent access. `Get` and `Put` both take a full write lock because they mutate list order. |

---

## Core Data Structure

```
head ↔ [most recent] ↔ ... ↔ [least recent] ↔ tail
```

The list is maintained such that the **most recently used** node is always right after `head`, and the **least recently used** node is always right before `tail`.

```
LRUCache[K, V] {
    capacity int
    cache    map[K]*Node[K,V]   ← O(1) lookup by key
    head     *Node[K,V]         ← sentinel (never holds real data)
    tail     *Node[K,V]         ← sentinel (never holds real data)
    mu       sync.RWMutex
}

Node[K, V] {
    key   K
    value V
    prev  *Node[K,V]
    next  *Node[K,V]
}
```

**Why store `key` in the Node?**
When `removeTail()` evicts the LRU node, we need to delete it from the map too. The node holds its own key so `delete(c.cache, lastNode.key)` works without a reverse lookup.

---

## File-by-File Breakdown

### `lru_cache.go` — The Cache

#### `NewLRUCache[K, V](capacity int) *LRUCache[K, V]`

Creates an empty cache and wires up the sentinel nodes:

```
head.next = tail
tail.prev = head
```

**Why sentinel nodes?**
Without them, every insert and remove must check `if node.prev == nil` and `if node.next == nil`. Sentinels guarantee every real node always has non-nil neighbors, so `addToHead`, `removeNode`, and `removeTail` are uniform 4-pointer operations with no conditionals.

---

#### `Get(key K) (V, bool)`

1. Acquires the **write lock** (not RLock — see note below).
2. Looks up `key` in the map. If absent, returns zero value + `false`.
3. If found, calls `moveToHead(node)` to mark it most recently used.
4. Returns `node.value, true`.

**Why a write lock for Get?**
`Get` calls `moveToHead`, which mutates the linked list (pointer rewiring). This is a write to shared state, so a full `Lock` is required. Using `RLock` here would be a data race even though it feels like a "read" operation from the caller's perspective.

**Time complexity:** O(1) — map lookup + 4-pointer list operation.

---

#### `Put(key K, value V)`

1. Acquires the **write lock**.
2. If `key` exists in the map: updates the node's value, calls `moveToHead`. Returns early (no eviction needed, count unchanged).
3. If `key` is new: creates a new node, inserts it into the map, calls `addToHead`.
4. If `len(cache) > capacity`: calls `removeTail()` to get the LRU node, then deletes it from the map.

**Why check length after insert (not before)?**
Inserting first and then evicting keeps the logic simple and linear — there is never a case where we evict and then fail to insert. The eviction always removes exactly one node when the count exceeds capacity by exactly one.

**Time complexity:** O(1) — map operations + 4-pointer list operations.

---

#### `addToHead(node *Node[K, V])`

Inserts `node` immediately after `head` (making it most recently used):

```
Before: head ↔ A ↔ ...
After:  head ↔ node ↔ A ↔ ...
```

Four pointer assignments — no conditionals, thanks to sentinels.

---

#### `removeNode(node *Node[K, V])`

Splices `node` out of the list by connecting its neighbors directly:

```
node.prev.next = node.next
node.next.prev = node.prev
```

Does not nil out `node.prev` or `node.next` — the caller immediately re-inserts the node (in `moveToHead`) or discards it (in `removeTail`).

---

#### `moveToHead(node *Node[K, V])`

`removeNode(node)` + `addToHead(node)`. Called on every cache hit (both `Get` and `Put` on an existing key).

---

#### `removeTail() *Node[K, V]`

Removes and returns the node just before `tail` — the least recently used real node. The caller (`Put`) deletes it from the map using `lastNode.key`.

---

#### `Size() int`

Returns `len(cache)` under a full write lock. Could use `RLock`, but keeping it consistent with other methods avoids reasoning about lock compatibility in calling code.

---

#### `Clear()`

Replaces the map with a new empty map and resets the sentinel links. The old nodes become unreachable and are garbage collected.

---

### `lru_cache_demo.go` — The Demo

```go
func Run() {
    lruCache := NewLRUCache[int, string](3)
    lruCache.Put(1, "value 1")  // [1]
    lruCache.Put(2, "value 2")  // [2, 1]
    lruCache.Put(3, "value 3")  // [3, 2, 1]
    lruCache.Get(1)             // [1, 3, 2]  — 1 moves to front
    lruCache.Get(2)             // [2, 1, 3]  — 2 moves to front
    lruCache.Put(4, "value 4")  // [4, 2, 1]  — 3 evicted (LRU)
    lruCache.Get(3)             // miss — "Value 3 was evicted"
    lruCache.Put(2, "updated value 2") // [2, 4, 1]  — update in place, move to front
}
```

---

## End-to-End Flow

```
NewLRUCache[int,string](3)
 └─ capacity = 3
 └─ head ↔ tail (empty list)

Put(1, "value 1")  → head ↔ [1] ↔ tail
Put(2, "value 2")  → head ↔ [2] ↔ [1] ↔ tail
Put(3, "value 3")  → head ↔ [3] ↔ [2] ↔ [1] ↔ tail

Get(1)             → moveToHead(1): head ↔ [1] ↔ [3] ↔ [2] ↔ tail
Get(2)             → moveToHead(2): head ↔ [2] ↔ [1] ↔ [3] ↔ tail

Put(4, "value 4")  → addToHead(4): head ↔ [4] ↔ [2] ↔ [1] ↔ [3] ↔ tail
                   → len(cache)=4 > capacity=3 → removeTail() → evict key 3
                   → head ↔ [4] ↔ [2] ↔ [1] ↔ tail

Get(3)             → miss (key 3 was evicted)

Put(2, "updated")  → key 2 exists → update value + moveToHead(2)
                   → head ↔ [2] ↔ [4] ↔ [1] ↔ tail
```

---

## Thread Safety

| Operation | Lock used | Why |
|---|---|---|
| `Get` | `mu.Lock()` (write) | `moveToHead` rewires list pointers — a write to shared state |
| `Put` | `mu.Lock()` (write) | Inserts/updates map + rewires list + possibly evicts |
| `Size` | `mu.Lock()` (write) | Conservative; consistent with other methods |
| `Clear` | `mu.Lock()` (write) | Replaces map and resets list — full write |

**Note:** A `sync.RWMutex` is held but only its write lock is ever used, because all public operations modify the list. The `RWMutex` is chosen over `sync.Mutex` to leave the door open for future read-only operations (e.g., `Peek` that reads without promoting to MRU) that could safely use `RLock`.

---

## Key Design Decisions & Trade-offs

| Decision | Alternative | Why this approach |
|---|---|---|
| Doubly linked list + map | Single map with timestamps | Map-only needs O(n) scan to find LRU. The list gives O(1) LRU identification via `tail.prev`. |
| Sentinel head/tail nodes | nil-terminated list | Eliminates all nil guards in pointer manipulation. Simpler, fewer bugs. |
| Store key in Node | Reverse map `*Node → K` | One map is simpler. Reverse map doubles memory and must stay in sync. |
| Full write lock on Get | RLock on Get | `Get` mutates list order. RLock would cause a data race. |
| Generics `[K comparable, V any]` | `interface{}` / `any` | Compile-time type safety, no type assertions, no boxing overhead for primitive keys. |
| Evict after insert | Evict before insert | Simpler: eviction condition is always `len > capacity`, exactly one node over. |
