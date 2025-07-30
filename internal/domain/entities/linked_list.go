package entities

import (
	"sync"
	"time"
)

// IssueNode represents a node in the linked list
type IssueNode struct {
	Issue *Issue
	Next  *IssueNode
	Prev  *IssueNode
}

// IssueLinkedList provides a high-performance doubly-linked list for issues
type IssueLinkedList struct {
	head    *IssueNode
	tail    *IssueNode
	size    int
	index   map[IssueID]*IssueNode // O(1) lookup by ID
	mutex   sync.RWMutex
	lastMod time.Time
}

// NewIssueLinkedList creates a new linked list for issues
func NewIssueLinkedList() *IssueLinkedList {
	return &IssueLinkedList{
		index:   make(map[IssueID]*IssueNode),
		lastMod: time.Now(),
	}
}

// Add inserts an issue at the end of the list (O(1))
func (list *IssueLinkedList) Add(issue *Issue) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	// Check if issue already exists
	if _, exists := list.index[issue.ID]; exists {
		return // Don't add duplicates
	}

	node := &IssueNode{Issue: issue}

	if list.head == nil {
		list.head = node
		list.tail = node
	} else {
		list.tail.Next = node
		node.Prev = list.tail
		list.tail = node
	}

	list.index[issue.ID] = node
	list.size++
	list.lastMod = time.Now()
}

// Remove deletes an issue from the list (O(1))
func (list *IssueLinkedList) Remove(issueID IssueID) bool {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	node, exists := list.index[issueID]
	if !exists {
		return false
	}

	// Update links
	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		list.head = node.Next
	}

	if node.Next != nil {
		node.Next.Prev = node.Prev
	} else {
		list.tail = node.Prev
	}

	delete(list.index, issueID)
	list.size--
	list.lastMod = time.Now()
	return true
}

// Get retrieves an issue by ID (O(1))
func (list *IssueLinkedList) Get(issueID IssueID) (*Issue, bool) {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	node, exists := list.index[issueID]
	if !exists {
		return nil, false
	}

	return node.Issue, true
}

// Update modifies an existing issue (O(1))
func (list *IssueLinkedList) Update(issue *Issue) bool {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	node, exists := list.index[issue.ID]
	if !exists {
		return false
	}

	node.Issue = issue
	list.lastMod = time.Now()
	return true
}

// GetAll returns all issues as a slice
func (list *IssueLinkedList) GetAll() []*Issue {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	issues := make([]*Issue, 0, list.size)
	current := list.head

	for current != nil {
		issues = append(issues, current.Issue)
		current = current.Next
	}

	return issues
}

// GetFiltered returns issues matching the given filter function
func (list *IssueLinkedList) GetFiltered(filter func(*Issue) bool) []*Issue {
	list.mutex.RLock()
	defer list.mutex.RUnlock()

	var issues []*Issue
	current := list.head

	for current != nil {
		if filter(current.Issue) {
			issues = append(issues, current.Issue)
		}
		current = current.Next
	}

	return issues
}

// Size returns the number of issues in the list
func (list *IssueLinkedList) Size() int {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.size
}

// IsEmpty checks if the list is empty
func (list *IssueLinkedList) IsEmpty() bool {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.size == 0
}

// LastModified returns the last modification time
func (list *IssueLinkedList) LastModified() time.Time {
	list.mutex.RLock()
	defer list.mutex.RUnlock()
	return list.lastMod
}

// Clear removes all issues from the list
func (list *IssueLinkedList) Clear() {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	list.head = nil
	list.tail = nil
	list.size = 0
	list.index = make(map[IssueID]*IssueNode)
	list.lastMod = time.Now()
}

// GetByStatus returns issues with the specified status
func (list *IssueLinkedList) GetByStatus(status string) []*Issue {
	return list.GetFiltered(func(issue *Issue) bool {
		return string(issue.Status) == status
	})
}

// GetByPriority returns issues with the specified priority
func (list *IssueLinkedList) GetByPriority(priority string) []*Issue {
	return list.GetFiltered(func(issue *Issue) bool {
		return string(issue.Priority) == priority
	})
}

// GetByAssignee returns issues assigned to the specified user
func (list *IssueLinkedList) GetByAssignee(assignee string) []*Issue {
	return list.GetFiltered(func(issue *Issue) bool {
		if issue.Assignee == nil {
			return assignee == ""
		}
		return issue.Assignee.Username == assignee
	})
}

// GetByType returns issues of the specified type
func (list *IssueLinkedList) GetByType(issueType string) []*Issue {
	return list.GetFiltered(func(issue *Issue) bool {
		return string(issue.Type) == issueType
	})
}
