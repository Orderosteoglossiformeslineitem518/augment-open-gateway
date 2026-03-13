package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	mathRand "math/rand"
	"strings"
	"time"
)

// MockDataGenerator 模拟数据生成器
type MockDataGenerator struct{}

// NewMockDataGenerator 创建新的模拟数据生成器
func NewMockDataGenerator() *MockDataGenerator {
	return &MockDataGenerator{}
}

// GenerateMachineId 生成机器ID
func (g *MockDataGenerator) GenerateMachineId(rng *mathRand.Rand) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff,
		rng.Uint32()&0xffff, rng.Uint64()&0xffffffffffff)
}

// GenerateHostname 生成主机名
func (g *MockDataGenerator) GenerateHostname(rng *mathRand.Rand) string {
	hostnames := []string{"MacBook-Pro", "MacBook-Air", "iMac", "Mac-mini", "MacBook"}
	suffix := rng.Intn(999) + 1
	return fmt.Sprintf("%s-%d", hostnames[rng.Intn(len(hostnames))], suffix)
}

// GenerateUsername 生成用户名
func (g *MockDataGenerator) GenerateUsername(rng *mathRand.Rand) string {
	usernames := []string{"admin", "user", "developer", "john", "jane", "alex", "chris", "sam"}
	return usernames[rng.Intn(len(usernames))]
}

// GenerateMacAddresses 生成MAC地址
func (g *MockDataGenerator) GenerateMacAddresses(rng *mathRand.Rand) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		rng.Intn(256), rng.Intn(256), rng.Intn(256),
		rng.Intn(256), rng.Intn(256), rng.Intn(256))
}

// GenerateKernelVersion 生成内核版本
func (g *MockDataGenerator) GenerateKernelVersion(rng *mathRand.Rand) string {
	major := rng.Intn(5) + 20
	minor := rng.Intn(10)
	patch := rng.Intn(10)
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

// GenerateDeviceId 生成设备ID
func (g *MockDataGenerator) GenerateDeviceId(rng *mathRand.Rand) string {
	return fmt.Sprintf("%016x", rng.Uint64())
}

// GenerateRandomHashWithSeed 使用种子生成随机哈希
func (g *MockDataGenerator) GenerateRandomHashWithSeed(rng *mathRand.Rand) string {
	bytes := make([]byte, 16)
	for i := range bytes {
		bytes[i] = byte(rng.Intn(256))
	}
	return hex.EncodeToString(bytes)
}

// GenerateOsMachineId 生成操作系统机器ID
func (g *MockDataGenerator) GenerateOsMachineId(rng *mathRand.Rand) string {
	return fmt.Sprintf("%08x%08x%08x%08x", rng.Uint32(), rng.Uint32(), rng.Uint32(), rng.Uint32())
}

// GenerateInode 生成inode号
func (g *MockDataGenerator) GenerateInode(rng *mathRand.Rand) string {
	return fmt.Sprintf("%d", rng.Uint64()&0x7fffffff)
}

// GenerateSshPublicKey 生成SSH公钥
func (g *MockDataGenerator) GenerateSshPublicKey(rng *mathRand.Rand) string {
	keyTypes := []string{"ssh-rsa", "ssh-ed25519", "ecdsa-sha2-nistp256"}
	keyType := keyTypes[rng.Intn(len(keyTypes))]
	keyData := make([]byte, 32)
	for i := range keyData {
		keyData[i] = byte(rng.Intn(256))
	}
	return fmt.Sprintf("%s %s", keyType, hex.EncodeToString(keyData))
}

// GenerateStorageUri 生成存储URI
func (g *MockDataGenerator) GenerateStorageUri(rng *mathRand.Rand) string {
	paths := []string{"/Users/user/Library", "/Applications", "/System/Library", "/usr/local"}
	return paths[rng.Intn(len(paths))]
}

// GenerateGpuInfo 生成GPU信息
func (g *MockDataGenerator) GenerateGpuInfo(rng *mathRand.Rand) string {
	gpus := []string{"Apple M4", "Apple M3", "Apple M2", "Apple M1", "Intel Iris"}
	return gpus[rng.Intn(len(gpus))]
}

// GenerateDiskLayout 生成磁盘布局
func (g *MockDataGenerator) GenerateDiskLayout(rng *mathRand.Rand) string {
	sizes := []string{"256GB", "512GB", "1TB", "2TB"}
	return fmt.Sprintf("SSD %s", sizes[rng.Intn(len(sizes))])
}

// GenerateSystemInfo 生成系统信息
func (g *MockDataGenerator) GenerateSystemInfo(rng *mathRand.Rand) string {
	return fmt.Sprintf("macOS %d.%d.%d", rng.Intn(3)+12, rng.Intn(10), rng.Intn(10))
}

// GenerateBiosInfo 生成BIOS信息
func (g *MockDataGenerator) GenerateBiosInfo(rng *mathRand.Rand) string {
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}
	return fmt.Sprintf("Apple BIOS %s", versions[rng.Intn(len(versions))])
}

// GenerateBaseboardInfo 生成主板信息
func (g *MockDataGenerator) GenerateBaseboardInfo(rng *mathRand.Rand) string {
	models := []string{"MacBookPro18,1", "MacBookPro18,2", "MacBookAir10,1", "iMac21,1"}
	return models[rng.Intn(len(models))]
}

// GenerateChassisInfo 生成机箱信息
func (g *MockDataGenerator) GenerateChassisInfo(rng *mathRand.Rand) string {
	types := []string{"Laptop", "Desktop", "All-in-One"}
	return types[rng.Intn(len(types))]
}

// GenerateAssetTag 生成资产标签
func (g *MockDataGenerator) GenerateAssetTag(rng *mathRand.Rand) string {
	return fmt.Sprintf("ASSET-%08d", rng.Intn(99999999))
}

// GenerateCpuFlags 生成CPU标志
func (g *MockDataGenerator) GenerateCpuFlags(rng *mathRand.Rand) string {
	flags := []string{"fpu", "vme", "de", "pse", "tsc", "msr", "pae", "mce"}
	selected := make([]string, rng.Intn(4)+2)
	for i := range selected {
		selected[i] = flags[rng.Intn(len(flags))]
	}
	return strings.Join(selected, " ")
}

// GenerateMemorySerials 生成内存序列号
func (g *MockDataGenerator) GenerateMemorySerials(rng *mathRand.Rand) string {
	return fmt.Sprintf("MEM%08X", rng.Uint32())
}

// GenerateUsbDeviceIds 生成USB设备ID
func (g *MockDataGenerator) GenerateUsbDeviceIds(rng *mathRand.Rand) string {
	return fmt.Sprintf("%04x:%04x", rng.Intn(0xffff), rng.Intn(0xffff))
}

// GenerateAudioDeviceIds 生成音频设备ID
func (g *MockDataGenerator) GenerateAudioDeviceIds(rng *mathRand.Rand) string {
	devices := []string{"Built-in Audio", "USB Audio", "Bluetooth Audio"}
	return devices[rng.Intn(len(devices))]
}

// GenerateHypervisorType 生成虚拟化类型
func (g *MockDataGenerator) GenerateHypervisorType(rng *mathRand.Rand) string {
	types := []string{"none", "vmware", "virtualbox", "parallels"}
	return types[rng.Intn(len(types))]
}

// GenerateBootTime 生成启动时间
func (g *MockDataGenerator) GenerateBootTime(rng *mathRand.Rand) string {
	// 生成最近几天内的启动时间
	days := rng.Intn(7)
	hours := rng.Intn(24)
	minutes := rng.Intn(60)
	bootTime := time.Now().AddDate(0, 0, -days).Add(-time.Duration(hours)*time.Hour - time.Duration(minutes)*time.Minute)
	return bootTime.Format("2006-01-02T15:04:05Z")
}

// GenerateSshKnownHosts 生成SSH已知主机
func (g *MockDataGenerator) GenerateSshKnownHosts(rng *mathRand.Rand) string {
	hosts := []string{"github.com", "gitlab.com", "bitbucket.org", "localhost"}
	return hosts[rng.Intn(len(hosts))]
}

// GenerateUuid 生成UUID
func (g *MockDataGenerator) GenerateUuid(rng *mathRand.Rand) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff,
		rng.Uint32()&0xffff, rng.Uint64()&0xffffffffffff)
}

// RandomHash 生成随机哈希值
func (g *MockDataGenerator) RandomHash() string {
	// 生成16字节随机数据
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		// 如果随机数生成失败，使用时间戳作为备选
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(randomBytes)
}

// CheckSum 计算字符串的SHA256哈希值
func (g *MockDataGenerator) CheckSum(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}
