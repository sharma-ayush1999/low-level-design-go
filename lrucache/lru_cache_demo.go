package lrucache

import "fmt"

func Run(){
	// Create a new cache with capacity 3
	lruCache := NewLRUCache[int, string](3)

	// Add some values
	lruCache.Put(1, "value 1")
	lruCache.Put(2, "value 2")
	lruCache.Put(3, "value 3")

	// Get values and print them
	if val, exists := lruCache.Get(1); exists {
		fmt.Println(val)
	}

	if val, exists := lruCache.Get(2); exists {
		fmt.Println(val)
	}

	//add new value
	lruCache.Put(4, "value 4")
	// Try to get the evicted value
	if val, exists := lruCache.Get(3); exists {
		fmt.Println(val)
	} else {
		fmt.Println("Value 3 was evicted")
	}

	// Get the newly added value
	if val, exists := lruCache.Get(4); exists {
		fmt.Println(val)
	}

	//update existing value
	lruCache.Put(2, "updated value 2")

	if val, exists := lruCache.Get(1); exists {
		fmt.Println(val)
	}

	if val, exists := lruCache.Get(2); exists {
		fmt.Println(val)
	}

}