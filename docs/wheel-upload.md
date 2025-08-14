# Wheel Upload 功能文档

## 概述

Wheel Upload 功能是 `llpkgstore` 的一个自动化工具，用于处理用户提交的 PR 请求，自动从 PyPI 下载 Python 库的 wheel 文件并上传到 GitHub Release。

## 工作流程

1. **用户提交 PR**：用户发现 `llgo get <库名>` 失败，提交 PR 说明缺少某个 Python 库的 wheel 文件
2. **GitHub Actions 触发**：系统自动检测 PR 并触发处理流程
3. **PyPI 搜索**：从 PyPI 搜索对应的库和最佳匹配的 wheel 文件
4. **文件下载**：下载选定的 wheel 文件到临时目录
5. **Release 管理**：在目标仓库创建或更新 Release
6. **文件上传**：将 wheel 文件上传到对应的 Release
7. **状态更新**：在 PR 中添加成功评论和状态更新

## 使用方法

### 1. 提交 PR 请求

用户需要按照以下格式提交 PR：

**PR 标题格式**：
```
Add missing wheel: <库名>
```

**PR 描述示例**：
```markdown
## Wheel Request

### Library Information
- **Library Name**: numpy
- **Version**: latest
- **Platform**: macos
- **Architecture**: x86_64

### Use Case
需要使用 numpy 进行数值计算和数组操作

### Additional Notes
无特殊要求
```

### 2. 环境变量配置

系统使用以下环境变量进行配置：

```bash
# GitHub 认证
GITHUB_TOKEN=your_github_token

# 目标仓库配置
TARGET_REPO_OWNER=Bigdata-shiyang
TARGET_REPO_NAME=test

# PyPI 配置
PYPI_BASE_URL=https://pypi.org/pypi
PYTHON_VERSION=3.12

# 源仓库配置（可选，有默认值）
GITHUB_REPOSITORY_OWNER=goplus
GITHUB_REPOSITORY=llpkgstore
```

### 3. 手动执行命令

如果需要手动执行 wheel 上传，可以使用以下命令：

```bash
cd cmd/llpkgstore
go build -o llpkgstore .
./llpkgstore wheel-upload <PR_NUMBER>
```

## 功能特性

### 1. 智能版本选择

- 自动选择最新稳定版本
- 支持指定版本要求
- 版本兼容性检查

### 2. 平台匹配

- 自动检测当前平台
- 优先选择平台特定的 wheel 文件
- 支持多平台文件上传

### 3. 架构支持

- 支持 x86_64 和 aarch64 架构
- 自动选择最佳匹配的架构
- 回退到通用架构

### 4. 错误处理

- 网络错误重试机制
- 详细的错误信息
- 失败状态更新

## Release 结构

上传的 wheel 文件会按照以下结构组织：

```
Bigdata-shiyang/test/releases
├── numpy/v1.24.3
│   ├── numpy-1.24.3-cp312-cp312-macosx_10_9_x86_64.whl
│   ├── numpy-1.24.3-cp312-cp312-linux_x86_64.whl
│   └── numpy-1.24.3-cp312-cp312-win_amd64.whl
├── pandas/v2.0.3
│   ├── pandas-2.0.3-cp312-cp312-macosx_10_9_x86_64.whl
│   └── pandas-2.0.3-cp312-cp312-linux_x86_64.whl
└── scipy/v1.11.1
    ├── scipy-1.11.1-cp312-cp312-macosx_10_9_x86_64.whl
    └── scipy-1.11.1-cp312-cp312-linux_x86_64.whl
```

## 支持的库类型

### 1. 纯 Python 库

- 无 C/C++ 扩展的库
- 通用 wheel 文件（any 平台）
- 例如：requests, urllib3

### 2. 带 C/C++ 扩展的库

- 包含编译的二进制文件
- 平台特定的 wheel 文件
- 例如：numpy, pandas, scipy

### 3. 特殊要求

- 某些库可能需要特定的 Python 版本
- 某些库可能有复杂的依赖关系
- 系统会尝试选择最佳兼容版本

## 故障排除

### 常见问题

1. **PR 标题格式错误**
   - 确保 PR 标题格式为：`Add missing wheel: <库名>`
   - 库名应该是有效的 Python 包名

2. **PyPI 搜索失败**
   - 检查库名是否正确
   - 确认库在 PyPI 上存在
   - 检查网络连接

3. **GitHub 权限错误**
   - 确保 GITHUB_TOKEN 有足够权限
   - 检查目标仓库的访问权限

4. **文件上传失败**
   - 检查文件大小限制
   - 确认 Release 创建成功
   - 检查网络连接

### 调试信息

系统会在 PR 评论中提供详细的调试信息，包括：

- 库名和版本信息
- 平台和架构信息
- 文件大小和 SHA256 摘要
- Release 链接
- 使用说明

## 扩展功能

### 1. 批量处理

未来可以支持批量处理多个库的请求，提高效率。

### 2. 依赖解析

自动解析库的依赖关系，确保所有依赖都被正确处理。

### 3. 版本管理

支持版本回滚、版本比较等高级功能。

### 4. 监控和统计

提供处理统计、成功率监控等功能。

## 贡献指南

如果您想为这个功能做出贡献，请：

1. Fork 项目仓库
2. 创建功能分支
3. 提交代码更改
4. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证。 