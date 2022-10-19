package wal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal/fbs/checkpoint"
)

// ErrEOFNotReached for indicating end of file not reached error
var ErrEOFNotReached = errors.New("expect to reach EOF, but actually didn't")

// readCheckpointV6 reads checkpoint file from a main file and 17 file parts.
// the main file stores:
//   - version
//   - checksum of each part file (17 in total)
//   - checksum of the main file itself
//     the first 16 files parts contain the trie nodes below the subtrieLevel
//     the last part file contains the top level trie nodes above the subtrieLevel and all the trie root nodes.
//
// it returns (tries, nil) if there was no error
// it returns (nil, os.ErrNotExist) if a certain file is missing, use (os.IsNotExist to check)
// it returns (nil, ErrEOFNotReached) if a certain part file is malformed
// it returns (nil, err) if running into any exception
func readCheckpointV6(headerFile *os.File, logger *zerolog.Logger) ([]*trie.MTrie, error) {
	// the full path of header file
	headerPath := headerFile.Name()
	dir, fileName := filepath.Split(headerPath)

	lg := logger.With().Str("checkpoint_file", headerPath).Logger()
	lg.Info().Msgf("reading v6 checkpoint file")

	// DONE
	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(headerPath, logger)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %w", err)
	}

	// ensure all checkpoint part file exists, might return os.ErrNotExist error
	// if a file is missing
	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
		return nil, fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	// TODO
	// TODO making number of goroutine configable for reading subtries, which can help us
	// test the code on machines that don't have as much RAM as EN by using fewer goroutines.
	subtrieNodes, err := readSubTriesConcurrently(dir, fileName, subtrieChecksums, &lg)
	if err != nil {
		return nil, fmt.Errorf("could not read subtrie from dir: %w", err)
	}

	lg.Info().Uint32("topsum", topTrieChecksum).
		Msg("finish reading all v6 subtrie files, start reading top level tries")

	// DONE
	tries, err := readTopLevelTries(dir, fileName, subtrieNodes, topTrieChecksum, &lg)
	if err != nil {
		return nil, fmt.Errorf("could not read top level nodes or tries: %w", err)
	}

	lg.Info().Msgf("finish reading all trie roots, trie root count: %v", len(tries))

	if len(tries) > 0 {
		first, last := tries[0], tries[len(tries)-1]
		logger.Info().
			Str("first_hash", first.RootHash().String()).
			Uint64("first_reg_count", first.AllocatedRegCount()).
			Str("last_hash", last.RootHash().String()).
			Uint64("last_reg_count", last.AllocatedRegCount()).
			Int("version", 6).
			Msg("checkpoint tries roots")
	}

	return tries, nil
}

// OpenAndReadCheckpointV6 open the checkpoint file and read it with readCheckpointV6
func OpenAndReadCheckpointV6(dir string, fileName string, logger *zerolog.Logger) (
	tries []*trie.MTrie,
	errToReturn error,
) {
	filepath := filePathCheckpointHeader(dir, fileName)

	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(file *os.File) {
		errToReturn = closeAndMergeError(file, errToReturn)
	}(f)

	return readCheckpointV6(f, logger)
}

func filePathCheckpointHeader(dir string, fileName string) string {
	return path.Join(dir, fileName)
}

func filePathSubTries(dir string, fileName string, index int) (string, string, error) {
	if index < 0 || index > (subtrieCount-1) {
		return "", "", fmt.Errorf("index must be between 0 to %v, but got %v", subtrieCount-1, index)
	}
	subTrieFileName := partFileName(fileName, index)
	return path.Join(dir, subTrieFileName), subTrieFileName, nil
}

func filePathTopTries(dir string, fileName string) (string, string) {
	topTriesFileName := partFileName(fileName, subtrieCount)
	return path.Join(dir, topTriesFileName), topTriesFileName
}

func partFileName(fileName string, index int) string {
	return fmt.Sprintf("%v.%03d", fileName, index)
}

func filePathPattern(dir string, fileName string) string {
	return fmt.Sprintf("%v*", filePathCheckpointHeader(dir, fileName))
}

// readCheckpointHeader takes a file path and returns subtrieChecksums and topTrieChecksum
// any error returned are exceptions
func readCheckpointHeader(filepath string, logger *zerolog.Logger) (
	checksumsOfSubtries []uint32,
	checksumOfTopTrie uint32,
	errToReturn error,
) {
	closable, err := os.Open(filepath)
	if err != nil {
		return nil, 0, fmt.Errorf("could not open header file: %w", err)
	}

	defer func(file *os.File) {
		evictErr := evictFileFromLinuxPageCache(file, false, logger)
		if evictErr != nil {
			logger.Warn().Msgf("failed to evict header file %s from Linux page cache: %s", filepath, evictErr)
			// No need to return this error because it's possible to continue normal operations.
		}
		errToReturn = closeAndMergeError(file, errToReturn)
	}(closable)

	var bufReader io.Reader = bufio.NewReaderSize(closable, defaultBufioReadSize)
	buf, err := io.ReadAll(bufReader)
	if err != nil {
		return nil, 0, err
	}

	checkpointHeaderBuf, checkpointHeaderCrc32Buf := buf[0:len(buf)-4], buf[len(buf)-4:]
	checkpointHeader := checkpoint.GetRootAsCheckpointHeader(checkpointHeaderBuf, 0)
	if checkpointHeader == nil {
		return nil, 0, fmt.Errorf("failed to decode CheckpointHeader")
	}

	// read the magic bytes and check version
	// TODO: skipping the magic number for now
	if checkpointHeader.Version() != VersionV6 {
		return nil, 0, fmt.Errorf("version mismatch - not VersionV6")
	}

	// read the subtrie count
	subtrieCount := checkpointHeader.SubtrieCount()

	subtrieChecksums := make([]uint32, subtrieCount)
	for i := uint16(0); i < subtrieCount; i++ {
		subtrieChecksums[i] = checkpointHeader.SubtrieChecksums(int(i))
	}

	// read top level trie checksum
	topTrieChecksum := checkpointHeader.TopLevelTrieChecksum()

	// calculate the actual checksum
	hasher := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	hasher.Write(checkpointHeaderBuf)
	actualSum := hasher.Sum32()

	// read the stored checksum, and compare with the actual sum
	expectedSum, err := decodeCRC32Sum(checkpointHeaderCrc32Buf)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode CheckpointHeaderCrc32")
	}

	if actualSum != expectedSum {
		return nil, 0, fmt.Errorf("invalid checksum in checkpoint header, expected %v, actual %v",
			expectedSum, actualSum)
	}

	return subtrieChecksums, topTrieChecksum, nil
}

// allPartFileExist check if all the part files of the checkpoint file exist
// it returns nil if all files exist
// it returns os.ErrNotExist if some file is missing, use (os.IsNotExist to check)
// it returns err if running into any exception
func allPartFileExist(dir string, fileName string, totalSubtrieFiles int) error {
	matched, err := findCheckpointPartFiles(dir, fileName)
	if err != nil {
		return fmt.Errorf("could not check all checkpoint part file exist: %w", err)
	}

	// header + subtrie files + top level file
	if len(matched) != 1+totalSubtrieFiles+1 {
		return fmt.Errorf("some checkpoint part file is missing. found part files %v. err :%w",
			matched, os.ErrNotExist)
	}

	return nil
}

// findCheckpointPartFiles returns a slice of file full paths of the part files for the checkpoint file
// with the given fileName under the given folder.
// - it return the matching part files, note it might not contains all the part files.
// - it return error if running any exception
func findCheckpointPartFiles(dir string, fileName string) ([]string, error) {
	pattern := filePathPattern(dir, fileName)
	matched, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("could not find checkpoint files: %w", err)
	}

	// build a lookup with matched
	lookup := make(map[string]struct{})
	for _, match := range matched {
		lookup[match] = struct{}{}
	}

	headerPath := filePathCheckpointHeader(dir, fileName)
	parts := make([]string, 0)
	// check header exists
	_, ok := lookup[headerPath]
	if ok {
		parts = append(parts, headerPath)
		delete(lookup, headerPath)
	}

	// check all subtrie parts
	for i := 0; i < subtrieCount; i++ {
		subtriePath, _, err := filePathSubTries(dir, fileName, i)
		if err != nil {
			return nil, err
		}
		_, ok := lookup[subtriePath]
		if ok {
			parts = append(parts, subtriePath)
			delete(lookup, subtriePath)
		}
	}

	// check top level trie part file
	toplevelPath, _ := filePathTopTries(dir, fileName)

	_, ok = lookup[toplevelPath]
	if ok {
		parts = append(parts, toplevelPath)
		delete(lookup, toplevelPath)
	}

	return parts, nil
}

type jobReadSubtrie struct {
	Index    int
	Checksum uint32
	Result   chan<- *resultReadSubTrie
}

type resultReadSubTrie struct {
	Nodes []*node.Node
	Err   error
}

func readSubTriesConcurrently(dir string, fileName string, subtrieChecksums []uint32, logger *zerolog.Logger) ([][]*node.Node, error) {

	numOfSubTries := len(subtrieChecksums)
	jobs := make(chan jobReadSubtrie, numOfSubTries)
	resultChs := make([]<-chan *resultReadSubTrie, numOfSubTries)

	// push all jobs into the channel
	for i, checksum := range subtrieChecksums {
		resultCh := make(chan *resultReadSubTrie)
		resultChs[i] = resultCh
		jobs <- jobReadSubtrie{
			Index:    i,
			Checksum: checksum,
			Result:   resultCh,
		}
	}
	close(jobs)

	// TODO: make nWorker configable
	nWorker := numOfSubTries // use as many worker as the jobs to read subtries concurrently
	for i := 0; i < nWorker; i++ {
		go func() {
			for job := range jobs {
				nodes, err := readCheckpointSubTrie(dir, fileName, job.Index, job.Checksum, logger)
				job.Result <- &resultReadSubTrie{
					Nodes: nodes,
					Err:   err,
				}
				close(job.Result)
			}
		}()
	}

	// reading job results in the same order as their indices
	nodesGroups := make([][]*node.Node, 0, len(resultChs))
	for i, resultCh := range resultChs {
		result := <-resultCh
		if result.Err != nil {
			return nil, fmt.Errorf("fail to read %v-th subtrie, trie: %w", i, result.Err)
		}

		nodesGroups = append(nodesGroups, result.Nodes)
	}

	return nodesGroups, nil
}

// subtrie file contains:
// 1. checkpoint version
// 2. nodes
// 3. node count
// 4. checksum
func readCheckpointSubTrie(dir string, fileName string, index int, checksum uint32, logger *zerolog.Logger) (
	subtrieRootNodes []*node.Node,
	errToReturn error,
) {
	filepath, _, err := filePathSubTries(dir, fileName, index)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(file *os.File) {
		evictErr := evictFileFromLinuxPageCache(file, false, logger)
		if evictErr != nil {
			logger.Warn().Msgf("failed to evict subtrie file %s from Linux page cache: %s", filepath, evictErr)
			// No need to return this error because it's possible to continue normal operations.
		}
		errToReturn = closeAndMergeError(file, errToReturn)
	}(f)

	bufAll, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	checkpointSubtrieBuf, checkpointSubtrieCrc32Buf := bufAll[0:len(bufAll)-4], bufAll[len(bufAll)-4:]
	checkpointSubtrie := checkpoint.GetRootAsCheckpointSubtrie(checkpointSubtrieBuf, 0)
	if checkpointSubtrie == nil {
		return nil, fmt.Errorf("failed to decode checkpointSubtrieBuf")
	}

	// valite the magic bytes and version
	if VersionV6 != checkpointSubtrie.Version() {
		return nil, fmt.Errorf("version mismatch")
	}

	nodesCount := checkpointSubtrie.NodeCount()
	expectedSum, _ := decodeCRC32Sum(checkpointSubtrieCrc32Buf)

	if checksum != expectedSum {
		return nil, fmt.Errorf("mismatch checksum in subtrie file. checksum from checkpoint header %v does not "+
			"match with the checksum in subtrie file %v", checksum, expectedSum)
	}

	// read file part index and verify
	scratch := make([]byte, 1024*4) // must not be less than 1024
	logging := logProgress(fmt.Sprintf("reading %v-th sub trie roots", index), int(nodesCount), logger)

	nodes := make([]*node.Node, nodesCount+1) //+1 for 0 index meaning nil
	for i := uint64(1); i <= nodesCount; i++ {
		var currNode checkpoint.Node
		ok := checkpointSubtrie.Nodes(&currNode, int(i-1))
		if !ok {
			return nil, fmt.Errorf("failed to load node")
		}
		data := currNode.DataBytes()
		currReader := bytes.NewReader(data)
		node, err := flattener.ReadNode(currReader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= i {
				return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
			}
			return nodes[nodeIndex], nil
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node %d: %w", i, err)
		}
		nodes[i] = node
		logging(i)
	}

	// calculate the actual checksum
	hasher := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	hasher.Write(checkpointSubtrieBuf)
	actualSum := hasher.Sum32()

	if actualSum != expectedSum {
		return nil, fmt.Errorf("invalid checksum in subtrie checkpoint, expected %v, actual %v",
			expectedSum, actualSum)
	}

	// since nodes[0] is always `nil`, returning a slice without nodes[0] could simplify the
	// implementation of getNodeByIndex
	return nodes[1:], nil
}

// 17th part file contains:
// 1. checkpoint version
// 2. subtrieNodeCount
// 3. top level nodes
// 4. trie roots
// 5. node count
// 6. trie count
// 7. checksum
func readTopLevelTries(dir string, fileName string, subtrieNodes [][]*node.Node, topTrieChecksum uint32, logger *zerolog.Logger) (
	rootTries []*trie.MTrie,
	errToReturn error,
) {
	filepath, _ := filePathTopTries(dir, fileName)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(file *os.File) {
		evictErr := evictFileFromLinuxPageCache(file, false, logger)
		if evictErr != nil {
			logger.Warn().Msgf("failed to evict top trie file %s from Linux page cache: %s", filepath, evictErr)
			// No need to return this error because it's possible to continue normal operations.
		}
		errToReturn = closeAndMergeError(file, errToReturn)
	}(file)

	bufAll, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	checkpointTopLevelBuf, checkpointTopLevelCrc32Buf := bufAll[0:len(bufAll)-4], bufAll[len(bufAll)-4:]
	checkpointTopLevel := checkpoint.GetRootAsCheckpointTopLevel(checkpointTopLevelBuf, 0)
	if checkpointTopLevel == nil {
		return nil, fmt.Errorf("failed to decode CheckpointTopLevel")
	}

	// read and validate magic bytes and version
	if VersionV6 != checkpointTopLevel.Version() { // TODO: no magic #
		return nil, fmt.Errorf("wrong version")
	}

	// read subtrie Node count and validate
	topLevelNodesCount, triesCount := checkpointTopLevel.NodeCount(), checkpointTopLevel.TrieCount()
	expectedSum, err := decodeCRC32Sum(checkpointTopLevelCrc32Buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CheckpointTopLevelCrc32")
	}

	if topTrieChecksum != expectedSum {
		return nil, fmt.Errorf("mismatch top trie checksum, header file has %v, toptrie file has %v",
			topTrieChecksum, expectedSum)
	}

	// read subtrie count and validate
	readSubtrieNodeCount := checkpointTopLevel.SubtrieNodeCount()

	totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)

	if readSubtrieNodeCount != totalSubTrieNodeCount {
		return nil, fmt.Errorf("mismatch subtrie node count, read from disk (%v), but got actual node count (%v)",
			readSubtrieNodeCount, totalSubTrieNodeCount)
	}

	topLevelNodes := make([]*node.Node, topLevelNodesCount+1) //+1 for 0 index meaning nil
	tries := make([]*trie.MTrie, triesCount)

	// Scratch buffer is used as temporary buffer that reader can read into.
	// Raw data in scratch buffer should be copied or converted into desired
	// objects before next Read operation.  If the scratch buffer isn't large
	// enough, a new buffer will be allocated.  However, 4096 bytes will
	// be large enough to handle almost all payloads and 100% of interim nodes.
	scratch := make([]byte, 1024*4) // must not be less than 1024

	// read the nodes from subtrie level to the root level
	for i := uint64(1); i <= topLevelNodesCount; i++ {
		var currNode checkpoint.Node
		ok := checkpointTopLevel.TopLevelNodes(&currNode, int(i-1))
		if !ok {
			return nil, fmt.Errorf("failed to load node")
		}
		data := currNode.DataBytes()
		currReader := bytes.NewReader(data)
		node, err := flattener.ReadNode(currReader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			if nodeIndex >= i+uint64(totalSubTrieNodeCount) {
				return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
			}

			return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
		})
		if err != nil {
			return nil, fmt.Errorf("cannot read node at index %d: %w", i, err)
		}

		topLevelNodes[i] = node
	}

	// read the trie root nodes
	for i := uint16(0); i < triesCount; i++ {
		var currTrie checkpoint.Trie
		checkpointTopLevel.TrieRoots(&currTrie, int(i))
		currReader := bytes.NewReader(currTrie.DataBytes())
		trie, err := flattener.ReadTrie(currReader, scratch, func(nodeIndex uint64) (*node.Node, error) {
			return getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, nodeIndex)
		})

		if err != nil {
			return nil, fmt.Errorf("cannot read root trie at index %d: %w", i, err)
		}
		tries[i] = trie
	}

	// calculate the actual checksum
	hasher := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	hasher.Write(checkpointTopLevelBuf)
	actualSum := hasher.Sum32()

	if actualSum != expectedSum {
		return nil, fmt.Errorf("invalid checksum in top level trie, expected %v, actual %v",
			expectedSum, actualSum)
	}

	return tries, nil
}

func computeTotalSubTrieNodeCount(groups [][]*node.Node) uint64 {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	return uint64(total)
}

// get a node by node index.
// Note: node index start from 1.
// subtries contains subtrie node groups. subtries[i][0] is NOT nil.
// topLevelNodes contains top level nodes. topLevelNodes[0] is nil.
// any error returned are exceptions
func getNodeByIndex(subtrieNodes [][]*node.Node, totalSubTrieNodeCount uint64, topLevelNodes []*node.Node, index uint64) (*node.Node, error) {
	if index == 0 {
		// item at index 0 is for nil
		return nil, nil
	}

	if index > totalSubTrieNodeCount {
		return getTopNodeByIndex(totalSubTrieNodeCount, topLevelNodes, index)
	}

	offset := index - 1 // index > 0, won't underflow
	for _, subtries := range subtrieNodes {
		if int(offset) < len(subtries) {
			return subtries[offset], nil
		}

		offset -= uint64(len(subtries))
	}

	return nil, fmt.Errorf("could not find node by index %v, totalSubTrieNodeCount %v", index, totalSubTrieNodeCount)
}

func getTopNodeByIndex(totalSubTrieNodeCount uint64, topLevelNodes []*node.Node, index uint64) (*node.Node, error) {
	nodePos := index - totalSubTrieNodeCount

	if nodePos >= uint64(len(topLevelNodes)) {
		return nil, fmt.Errorf("can not find node by index %v, nodePos >= len(topLevelNodes) => (%v > %v)",
			index, nodePos, len(topLevelNodes))
	}

	return topLevelNodes[nodePos], nil
}
