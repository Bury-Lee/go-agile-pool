package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"runtime"
)

// GetStack 获取调用堆栈信息，包含文件名、行号、PC地址和源代码内容
//
// 参数说明：
//
//	skip: 跳过的调用栈帧数。设置为 1 表示跳过 GetStack 函数本身，
//	      设置为 2 还会跳过调用 GetStack 的函数，以此类推
//
// 返回值：
// d:\开源项目\AStoryForge\Server\common\enter_test.go
//
//	返回格式化的堆栈信息字节切片，每帧包含：
//	  - 文件路径:行号 (PC地址)
//	  - 函数名: 该行的源代码内容
//
// 输出示例：
//
//	/home/user/main.go:25 (0x45a6f8)
//	    main.main: fmt.Println("hello world")
//	/home/user/main.go:30 (0x45a8a2)
//	    main.testFunc
//
// 注意事项：
//  1. 需要确保源代码文件可读，否则会显示 "???" 作为源代码行
//  2. 连续相同文件的调用不会重复输出源代码行，只显示函数名
//  3. 该函数会进行文件 I/O 操作，频繁调用可能影响性能
//  4. 适用于开发调试环境，生产环境建议使用 runtime.Stack
func Stack(skip int) []byte {
	// 创建字节缓冲区，用于高效拼接字符串
	buf := new(bytes.Buffer)
	// 记录上一个处理的文件名，用于避免重复输出相同文件的源代码
	var lastFile string
	// 当无法读取源代码时的占位符
	dunno := "???"

	// 从 skip 层开始遍历调用栈，直到栈顶
	for i := skip; ; i++ {
		// 获取调用栈信息：程序计数器PC、文件名、行号、是否有效
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			// 已经到达栈顶，退出循环
			break
		}

		// 输出当前位置：文件路径、行号和程序计数器地址（十六进制）
		// 示例：/path/to/main.go:42 (0x4c30f5)
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)

		// 判断是否与上一个文件相同，避免重复读取同一文件
		if file != lastFile {
			// 读取该行对应的源代码内容
			// line-1 是因为行号从1开始，而切片索引从0开始
			sourceLine, err := readNthLine(file, line-1)
			if err != nil {
				// 读取失败（文件不存在、权限不足等），使用占位符
				sourceLine = dunno
			}
			// 输出函数名和源代码行，使用制表符缩进
			// 示例：	main.main: fmt.Println("hello")
			fmt.Fprintf(buf, "\t%s: %s\n", function(pc), sourceLine)
			// 更新最后处理的文件名
			lastFile = file
		} else {
			// 同一文件的连续调用，只输出函数名，不再重复输出源代码
			// 示例：	main.helperFunc
			fmt.Fprintf(buf, "\t%s\n", function(pc))
		}
	}
	// 返回缓冲区中的字节数据
	return buf.Bytes()
}

// function 根据程序计数器（PC）获取函数名称
//
// 参数说明：
//
//	pc: 程序计数器地址，通常从 runtime.Caller 获取
//
// 返回值：
//
//	返回完整的函数名（包含包路径），格式如 "main.main" 或 "fmt.Println"
//	如果找不到对应的函数信息，返回 "unknown"
//
// 实现原理：
//
//	runtime.FuncForPC 通过 PC 地址查找对应的函数元信息
func function(pc uintptr) string {
	// 根据程序计数器获取对应的函数信息
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		// 无法找到对应函数（可能已被优化或内联）
		return "unknown"
	}
	// 返回函数的完整名称
	return fn.Name()
}

// readNthLine 读取文件的指定行内容
//
// 参数说明：
//
//	file: 要读取的文件路径（绝对路径或相对路径）
//	n:    要读取的行号索引（从0开始，0表示第一行）
//
// 返回值：
//
//	成功：返回该行的字符串内容和 nil 错误
//	失败：返回空字符串和具体的错误信息
//
// 可能返回的错误：
//   - 文件打开失败（文件不存在、无权限等）
//   - 文件读取过程中的 I/O 错误
//   - 指定的行号超出文件总行数
//
// 性能说明：
//
//	该函数每次调用都会打开文件并从第一行开始扫描，
//	频繁调用时性能较差，建议配合缓存使用
//
// 示例：
//
//	line, err := readNthLine("/path/to/file.go", 41) // 读取第42行（因为索引从0开始）
//	if err != nil {
//	    log.Printf("读取失败: %v", err)
//	}
func readNthLine(file string, n int) (string, error) {
	// 打开文件
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	// 确保函数退出时关闭文件句柄，防止资源泄漏
	defer f.Close()

	// 创建带缓冲的扫描器，提高读取效率
	scanner := bufio.NewScanner(f)
	// 当前扫描到的行号（从0开始）
	lineNum := 0
	// 逐行扫描文件
	for scanner.Scan() {
		// 找到目标行
		if lineNum == n {
			// 返回该行的文本内容
			return scanner.Text(), nil
		}
		lineNum++
	}
	// 扫描过程中发生错误（非EOF错误）或文件行数不足
	// scanner.Err() 可能返回 io.EOF 或其他读取错误
	return "", scanner.Err()
}
