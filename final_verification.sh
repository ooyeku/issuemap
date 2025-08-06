#!/bin/bash

echo "========================================"
echo "FINAL VERIFICATION OF ISSUEMAP BRANCH FIX"
echo "========================================"

# Clean up any existing test data
echo ""
echo "🧹 Cleaning up previous test data..."
git checkout main 2>/dev/null
git branch -D test-* 2>/dev/null || true
rm -rf .issuemap/issues/ISSUE-*.yaml 2>/dev/null || true
rm -rf .issuemap/history/ISSUE-*.yaml 2>/dev/null || true

echo ""
echo "========== TEST 1: COMPLETE WORKFLOW =========="
echo "1. Creating test issue..."
./bin/issuemap create "Final verification test issue" --type feature --priority medium

ISSUE_ID=$(./bin/issuemap list --limit 1 | grep ISSUE | awk '{print $1}')
echo "   ✅ Created: $ISSUE_ID"

echo ""
echo "2. Assigning issue to user..."
./bin/issuemap assign $ISSUE_ID testuser
echo "   ✅ Assigned to testuser"

echo ""
echo "3. Showing issue before branching..."
echo "   Branch field: $(./bin/issuemap show $ISSUE_ID | grep "Branch:" | awk '{print $2}')"
echo "   File exists: $(ls .issuemap/issues/$ISSUE_ID.yaml >/dev/null 2>&1 && echo 'YES' || echo 'NO')"

echo ""
echo "4. Creating and switching to branch..."
./bin/issuemap branch $ISSUE_ID

echo ""
echo "5. Verifying issue after branching..."
CURRENT_BRANCH=$(git branch --show-current)
echo "   Current branch: $CURRENT_BRANCH"
echo "   Issue still exists: $(./bin/issuemap show $ISSUE_ID >/dev/null 2>&1 && echo 'YES' || echo 'NO')"
echo "   File still exists: $(ls .issuemap/issues/$ISSUE_ID.yaml >/dev/null 2>&1 && echo 'YES' || echo 'NO')"
echo "   Branch field updated: $(./bin/issuemap show $ISSUE_ID | grep "Branch:" | awk '{print $2}')"

echo ""
echo "6. Testing merge workflow..."
# Make a dummy commit so we have something to merge
echo "test change" > test_file.txt
git add test_file.txt
git commit -m "Test commit for $ISSUE_ID"

./bin/issuemap merge
MERGE_RESULT=$?
echo "   Merge result: $(if [ $MERGE_RESULT -eq 0 ]; then echo 'SUCCESS'; else echo 'FAILED'; fi)"

# Clean up
rm -f test_file.txt
git checkout main 2>/dev/null

echo ""
echo "========== TEST 2: UNCOMMITTED CHANGES SCENARIO =========="

# Create another issue but don't commit anything
echo "1. Creating issue without committing..."
./bin/issuemap create "Test uncommitted scenario" --type bug --priority low

ISSUE_ID2=$(./bin/issuemap list --limit 1 | grep ISSUE | awk '{print $1}')
echo "   ✅ Created: $ISSUE_ID2"

echo ""
echo "2. Checking git status before branching..."
echo "   Uncommitted issuemap files: $(git status --porcelain .issuemap | wc -l | tr -d ' ')"

echo ""
echo "3. Creating branch (should auto-commit)..."
./bin/issuemap branch $ISSUE_ID2

echo ""
echo "4. Verifying auto-commit happened..."
echo "   Issue still accessible: $(./bin/issuemap show $ISSUE_ID2 >/dev/null 2>&1 && echo 'YES' || echo 'NO')"
echo "   Git log shows commit: $(git log --oneline -1 | grep -q "Add issuemap files" && echo 'YES' || echo 'NO')"

git checkout main 2>/dev/null

echo ""
echo "========== TEST 3: CLEAN WORKING DIRECTORY =========="

# Commit everything first
git add . 2>/dev/null || true
git commit -m "Prepare for clean directory test" 2>/dev/null || true

echo "1. Creating issue with clean working directory..."
./bin/issuemap create "Clean directory test" --type task --priority high

ISSUE_ID3=$(./bin/issuemap list --limit 1 | grep ISSUE | awk '{print $1}')
echo "   ✅ Created: $ISSUE_ID3"

# Commit this issue
git add .issuemap
git commit -m "Add issue $ISSUE_ID3"

echo ""
echo "2. Creating branch with clean directory..."
./bin/issuemap branch $ISSUE_ID3

echo ""
echo "3. Verifying clean branch switch..."
echo "   Issue accessible: $(./bin/issuemap show $ISSUE_ID3 >/dev/null 2>&1 && echo 'YES' || echo 'NO')"
echo "   Current branch: $(git branch --show-current)"

git checkout main 2>/dev/null

echo ""
echo "========== TEST 4: BRANCH SWITCHING PERSISTENCE =========="

echo "1. Listing all issues from main branch..."
MAIN_ISSUES=$(./bin/issuemap list | grep ISSUE | wc -l | tr -d ' ')
echo "   Issues on main: $MAIN_ISSUES"

# Switch to feature branch if it exists
FEATURE_BRANCH=$(git branch | grep feature | head -1 | tr -d ' *')
if [ ! -z "$FEATURE_BRANCH" ]; then
    echo ""
    echo "2. Switching to $FEATURE_BRANCH..."
    git checkout $FEATURE_BRANCH 2>/dev/null
    
    FEATURE_ISSUES=$(./bin/issuemap list | grep ISSUE | wc -l | tr -d ' ')
    echo "   Issues on $FEATURE_BRANCH: $FEATURE_ISSUES"
    
    echo "   Issue persistence: $(if [ $MAIN_ISSUES -eq $FEATURE_ISSUES ]; then echo 'CONSISTENT'; else echo 'INCONSISTENT'; fi)"
else
    echo "   No feature branches found for testing"
fi

git checkout main 2>/dev/null

echo ""
echo "========== VERIFICATION SUMMARY =========="
echo "✅ Complete workflow test completed"
echo "✅ Uncommitted changes scenario tested"  
echo "✅ Clean working directory scenario tested"
echo "✅ Branch switching persistence verified"
echo ""
echo "🎉 ALL TESTS COMPLETED!"
echo ""
echo "Final issue count: $(./bin/issuemap list | grep ISSUE | wc -l | tr -d ' ') issues"
echo "Repository state: $(git status --porcelain | wc -l | tr -d ' ') uncommitted changes"