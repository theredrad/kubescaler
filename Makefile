generate_mocks:
	mockgen -mock_names Provider=MockNodePoolProvider -package mocks -source=./nodepoolmanager/nodepoolmanager.go NodePoolManager > ./mocks/nodepoolmanager.go