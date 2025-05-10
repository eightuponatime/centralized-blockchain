package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/shopspring/decimal"
)

// Transaction структура для транзакции
type Transaction struct {
	TrainerId   int             `json:"trainerId"`
	ClientId    int             `json:"clientId"`
	PaymentDate time.Time       `json:"paymentDate"`
	Amount      decimal.Decimal `json:"amount"`
}

// Block структура для блока
type Block struct {
	Index       int
	Hash        []byte
	Transaction *Transaction
	PrevHash    []byte
	Timestamp   time.Time
	HMAC        []byte
}

// BlockIndex структура для индекса
type BlockIndex struct {
	ClientId  int
	TrainerId int
	Offset    int64
	BlockHash []byte
	HMAC      []byte
}

// Index хранит индексы для быстрого поиска
type Index struct {
	ByClientId        map[int][]BlockIndex
	ByClientTrainerId map[string][]BlockIndex
}

// NewIndex создаёт новый индекс
func NewIndex() *Index {
	return &Index{
		ByClientId:        make(map[int][]BlockIndex),
		ByClientTrainerId: make(map[string][]BlockIndex),
	}
}

// Add добавляет блок в индекс
func (idx *Index) Add(block *Block, offset int64) {
	clientId := block.Transaction.ClientId
	trainerId := block.Transaction.TrainerId
	blockIndex := BlockIndex{
		ClientId:  clientId,
		TrainerId: trainerId,
		Offset:    offset,
		BlockHash: block.Hash,
		HMAC:      block.HMAC,
	}
	idx.ByClientId[clientId] = append(idx.ByClientId[clientId], blockIndex)
	key := fmt.Sprintf("%d:%d", clientId, trainerId)
	idx.ByClientTrainerId[key] = append(idx.ByClientTrainerId[key], blockIndex)
}

// Сериализация транзакции
func (t *Transaction) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	// Записываем данные транзакции
	data := new(bytes.Buffer)
	if err := binary.Write(data, binary.BigEndian, int32(t.TrainerId)); err != nil {
		return nil, err
	}
	if err := binary.Write(data, binary.BigEndian, int32(t.ClientId)); err != nil {
		return nil, err
	}
	if err := binary.Write(data, binary.BigEndian, t.PaymentDate.Unix()); err != nil {
		return nil, err
	}
	amountStr := t.Amount.String()
	if len(amountStr) > 32 {
		return nil, fmt.Errorf("amount too long")
	}
	amountBytes := make([]byte, 32)
	copy(amountBytes, amountStr)
	if _, err := data.Write(amountBytes); err != nil {
		return nil, err
	}
	// Записываем длину данных транзакции
	if err := binary.Write(&buf, binary.BigEndian, int32(data.Len())); err != nil {
		return nil, err
	}
	// Записываем сами данные
	if _, err := buf.Write(data.Bytes()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Десериализация транзакции
func DeserializeTransaction(reader io.Reader) (*Transaction, error) {
	// Читаем длину транзакции
	var length int32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read transaction length: %v", err)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read transaction data: %v", err)
	}
	buf := bytes.NewReader(data)
	var tx Transaction
	var trainerId, clientId int32
	var timestamp int64

	if err := binary.Read(buf, binary.BigEndian, &trainerId); err != nil {
		return nil, fmt.Errorf("failed to read trainerId: %v", err)
	}
	tx.TrainerId = int(trainerId)

	if err := binary.Read(buf, binary.BigEndian, &clientId); err != nil {
		return nil, fmt.Errorf("failed to read clientId: %v", err)
	}
	tx.ClientId = int(clientId)

	if err := binary.Read(buf, binary.BigEndian, &timestamp); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %v", err)
	}
	tx.PaymentDate = time.Unix(timestamp, 0)

	amountBytes := make([]byte, 32)
	if _, err := buf.Read(amountBytes); err != nil {
		return nil, fmt.Errorf("failed to read amount: %v", err)
	}
	amountStr := string(bytes.TrimRight(amountBytes, "\x00"))
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %v", err)
	}
	tx.Amount = amount

	return &tx, nil
}

// Сериализация блока
func (b *Block) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	// Записываем длину блока
	data, err := b.serializeWithoutLength()
	if err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, int32(len(data))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Сериализация блока без длины
func (b *Block) serializeWithoutLength() ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, int32(b.Index)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, b.Timestamp.Unix()); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b.PrevHash); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b.Hash); err != nil {
		return nil, err
	}
	txData, err := b.Transaction.Serialize()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(txData); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b.HMAC); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Десериализация блока
func DeserializeBlock(reader io.Reader) (*Block, error) {
	// Читаем длину блока
	var length int32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read block length: %v", err)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("failed to read block data: %v", err)
	}
	buf := bytes.NewReader(data)
	var block Block
	var index int32
	var timestamp int64

	if err := binary.Read(buf, binary.BigEndian, &index); err != nil {
		return nil, fmt.Errorf("failed to read index: %v", err)
	}
	block.Index = int(index)

	if err := binary.Read(buf, binary.BigEndian, &timestamp); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %v", err)
	}
	block.Timestamp = time.Unix(timestamp, 0)

	block.PrevHash = make([]byte, 32)
	if _, err := buf.Read(block.PrevHash); err != nil {
		return nil, fmt.Errorf("failed to read prevHash: %v", err)
	}

	block.Hash = make([]byte, 32)
	if _, err := buf.Read(block.Hash); err != nil {
		return nil, fmt.Errorf("failed to read hash: %v", err)
	}

	tx, err := DeserializeTransaction(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %v", err)
	}
	block.Transaction = tx

	block.HMAC = make([]byte, 32)
	if _, err := buf.Read(block.HMAC); err != nil {
		return nil, fmt.Errorf("failed to read HMAC: %v", err)
	}

	return &block, nil
}

// Вычисление HMAC для блока
func (b *Block) CalculateHMAC(secretKey []byte) ([]byte, error) {
	data, err := b.serializeWithoutHMAC()
	if err != nil {
		return nil, err
	}
	h := hmac.New(sha256.New, secretKey)
	h.Write(data)
	return h.Sum(nil), nil
}

// Сериализация блока без HMAC
func (b *Block) serializeWithoutHMAC() ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, int32(b.Index)); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, b.Timestamp.Unix()); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b.PrevHash); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b.Hash); err != nil {
		return nil, err
	}
	txData, err := b.Transaction.Serialize()
	if err != nil {
		return nil, err
	}
	if _, err := buf.Write(txData); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Проверка HMAC блока
func (b *Block) VerifyHMAC(secretKey []byte) (bool, error) {
	expectedHMAC, err := b.CalculateHMAC(secretKey)
	if err != nil {
		return false, err
	}
	return hmac.Equal(b.HMAC, expectedHMAC), nil
}

// Создание нового блока
func createNewBlock(transaction *Transaction, prevHash []byte, prevIndex int, secretKey []byte) (*Block, error) {
	txData, err := transaction.Serialize()
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	h.Write(txData)
	h.Write(prevHash)

	block := &Block{
		Index:       prevIndex + 1,
		Hash:        h.Sum(nil),
		Transaction: transaction,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}

	// Вычисляем HMAC
	block.HMAC, err = block.CalculateHMAC(secretKey)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// Сохранение блока в файл с возвратом позиции
func addBlockToFile(block *Block, filename string) (int64, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	offset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to seek: %v", err)
	}
	data, err := block.Serialize()
	if err != nil {
		return 0, fmt.Errorf("failed to serialize block: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		return 0, fmt.Errorf("failed to write block: %v", err)
	}
	return offset, nil
}

// Сохранение индекса
func saveIndex(index *Index, indexFile string) error {
	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("failed to create index file: %v", err)
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	return enc.Encode(index)
}

// Загрузка индекса
func loadIndex(indexFile string) (*Index, error) {
	f, err := os.Open(indexFile)
	if os.IsNotExist(err) {
		return NewIndex(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %v", err)
	}
	defer f.Close()
	var index Index
	dec := gob.NewDecoder(f)
	err = dec.Decode(&index)
	if err != nil {
		return nil, fmt.Errorf("failed to decode index: %v", err)
	}
	if index.ByClientId == nil {
		index.ByClientId = make(map[int][]BlockIndex)
	}
	if index.ByClientTrainerId == nil {
		index.ByClientTrainerId = make(map[string][]BlockIndex)
	}
	return &index, nil
}

// Поиск блоков по ClientId
func findBlocksByClientId(clientId int, filename string, index *Index, secretKey []byte) ([]*Block, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	var blocks []*Block
	for _, idx := range index.ByClientId[clientId] {
		fmt.Printf("Reading block at offset %d\n", idx.Offset)
		if _, err := f.Seek(idx.Offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to offset %d: %v", idx.Offset, err)
		}
		block, err := DeserializeBlock(f)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %v", idx.Offset, err)
		}
		// Проверяем HMAC
		valid, err := block.VerifyHMAC(secretKey)
		if err != nil {
			return nil, fmt.Errorf("HMAC verification failed for block %d: %v", block.Index, err)
		}
		if !valid {
			return nil, fmt.Errorf("invalid HMAC for block at index %d", block.Index)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// Асинхронный поиск блоков по ClientId
func findBlocksByClientIdAsync(clientId int, filename string, index *Index, secretKey []byte, results chan<- *Block, errors chan<- error) {
	blocks, err := findBlocksByClientId(clientId, filename, index, secretKey)
	if err != nil {
		errors <- err
		return
	}
	for _, block := range blocks {
		results <- block
	}
}

// Поиск блоков по ClientId и TrainerId
func findBlocksByClientTrainerId(clientId, trainerId int, filename string, index *Index, secretKey []byte) ([]*Block, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	var blocks []*Block
	key := fmt.Sprintf("%d:%d", clientId, trainerId)
	for _, idx := range index.ByClientTrainerId[key] {
		fmt.Printf("Reading block at offset %d\n", idx.Offset)
		if _, err := f.Seek(idx.Offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to offset %d: %v", idx.Offset, err)
		}
		block, err := DeserializeBlock(f)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize block at offset %d: %v", idx.Offset, err)
		}
		// Проверяем HMAC
		valid, err := block.VerifyHMAC(secretKey)
		if err != nil {
			return nil, fmt.Errorf("HMAC verification failed for block %d: %v", block.Index, err)
		}
		if !valid {
			return nil, fmt.Errorf("invalid HMAC for block at index %d", block.Index)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// Проверка целостности цепочки
func verifyChain(filename string, secretKey []byte) (bool, error) {
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		fmt.Println("Blockchain file does not exist, starting with empty chain")
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	var prevBlock *Block
	for i := 1; ; i++ {
		block, err := DeserializeBlock(f)
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to deserialize block %d: %v", i, err)
		}
		if block.Index != i {
			return false, fmt.Errorf("invalid index %d, expected %d", block.Index, i)
		}
		valid, err := block.VerifyHMAC(secretKey)
		if err != nil || !valid {
			return false, fmt.Errorf("invalid HMAC for block %d", block.Index)
		}
		if prevBlock != nil && !bytes.Equal(block.PrevHash, prevBlock.Hash) {
			return false, fmt.Errorf("chain broken at index %d", block.Index)
		}
		prevBlock = block
	}
	return true, nil
}

// Инициализация транзакции
func initTransaction(trainerId, clientId int, amount float64) *Transaction {
	return &Transaction{
		TrainerId:   trainerId,
		ClientId:    clientId,
		PaymentDate: time.Now(),
		Amount:      decimal.NewFromFloat(amount),
	}
}

func main() {
	// Секретный ключ для HMAC
	secretKey := []byte("my-secret-key-1234567890") // Храните ключ безопасно!

	// Инициализируем индекс
	filename := "blockchain.dat"
	indexFile := "index.dat"
	index, err := loadIndex(indexFile)
	if err != nil {
		fmt.Printf("Error loading index: %v\n", err)
		return
	}

	// Проверяем целостность цепочки перед началом
	valid, err := verifyChain(filename, secretKey)
	if err != nil {
		fmt.Printf("Chain verification failed: %v\n", err)
		return
	}
	if !valid {
		fmt.Println("Chain is invalid")
		return
	}
	fmt.Println("Chain is valid")

	// Создаём генезис-хеш
	hash := sha256.New()
	hash.Write([]byte("genesis"))
	genesisPrevHash := hash.Sum(nil)

	// Создаём транзакцию для генезис-блока
	genesisTransaction := initTransaction(1, 2, 125124.55)

	// Создаём генезис-блок
	genesisBlock, err := createNewBlock(genesisTransaction, genesisPrevHash, 0, secretKey)
	if err != nil {
		fmt.Printf("Error creating genesis block: %v\n", err)
		return
	}

	// Сохраняем генезис-блок и обновляем индекс
	offset, err := addBlockToFile(genesisBlock, filename)
	if err != nil {
		fmt.Printf("Error saving genesis block: %v\n", err)
		return
	}
	index.Add(genesisBlock, offset)

	// Сохраняем индекс
	err = saveIndex(index, indexFile)
	if err != nil {
		fmt.Printf("Error saving index: %v\n", err)
		return
	}

	// Выводим информацию о генезис-блоке
	fmt.Printf("Genesis Block:\n")
	fmt.Printf("  Index: %d\n", genesisBlock.Index)
	fmt.Printf("  Hash: %x\n", genesisBlock.Hash)
	fmt.Printf("  PrevHash: %x\n", genesisBlock.PrevHash)
	fmt.Printf("  HMAC: %x\n", genesisBlock.HMAC)
	fmt.Printf("  Transaction: %+v\n", genesisBlock.Transaction)

	// Создаём вторую транзакцию и блок
	secondTransaction := initTransaction(3, 2, 2000.75)
	secondBlock, err := createNewBlock(secondTransaction, genesisBlock.Hash, genesisBlock.Index, secretKey)
	if err != nil {
		fmt.Printf("Error creating second block: %v\n", err)
		return
	}

	// Сохраняем второй блок и обновляем индекс
	offset, err = addBlockToFile(secondBlock, filename)
	if err != nil {
		fmt.Printf("Error saving second block: %v\n", err)
		return
	}
	index.Add(secondBlock, offset)

	// Сохраняем индекс
	err = saveIndex(index, indexFile)
	if err != nil {
		fmt.Printf("Error saving index: %v\n", err)
		return
	}

	// Выводим информацию о втором блоке
	fmt.Printf("\nSecond Block:\n")
	fmt.Printf("  Index: %d\n", secondBlock.Index)
	fmt.Printf("  Hash: %x\n", secondBlock.Hash)
	fmt.Printf("  PrevHash: %x\n", secondBlock.PrevHash)
	fmt.Printf("  HMAC: %x\n", secondBlock.HMAC)
	fmt.Printf("  Transaction: %+v\n", secondBlock.Transaction)

	// Пример поиска блоков по ClientId
	blocks, err := findBlocksByClientId(2, filename, index, secretKey)
	if err != nil {
		fmt.Printf("Error finding blocks by ClientId: %v\n", err)
		return
	}
	fmt.Printf("\nBlocks for ClientId 2:\n")
	for _, block := range blocks {
		fmt.Printf("  Block Index: %d, Transaction: %+v, HMAC Valid: %v\n", block.Index, block.Transaction, true)
	}

	// Пример поиска блоков по ClientId и TrainerId
	blocks, err = findBlocksByClientTrainerId(2, 1, filename, index, secretKey)
	if err != nil {
		fmt.Printf("Error finding blocks by ClientId+TrainerId: %v\n", err)
		return
	}
	fmt.Printf("\nBlocks for ClientId 2 and TrainerId 1:\n")
	for _, block := range blocks {
		fmt.Printf("  Block Index: %d, Transaction: %+v, HMAC Valid: %v\n", block.Index, block.Transaction, true)
	}

	// Пример асинхронного поиска по нескольким ClientId
	clientIds := []int{2, 3}
	results := make(chan *Block, 100)
	errors := make(chan error, len(clientIds))
	for _, clientId := range clientIds {
		go findBlocksByClientIdAsync(clientId, filename, index, secretKey, results, errors)
	}
	var asyncBlocks []*Block
	for i := 0; i < len(clientIds); i++ {
		select {
		case block := <-results:
			asyncBlocks = append(asyncBlocks, block)
		case err := <-errors:
			fmt.Printf("Error in async search: %v\n", err)
			return
		}
	}
	fmt.Printf("\nAsync search results for ClientIds %v:\n", clientIds)
	for _, block := range asyncBlocks {
		fmt.Printf("  Block Index: %d, Transaction: %+v, HMAC Valid: %v\n", block.Index, block.Transaction, true)
	}
}
