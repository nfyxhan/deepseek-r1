
### 代码小白都能快速本地化部署deepseek了

## 安装步骤
### 1. 下载[ollama](https://github.com/ollama/ollama/releases/download/v0.3.1/OllamaSetup.exe), 根据提示安装

### 2. 下载模型 (通过 win + R 打开cmd，输入以下命令)
```
ollama pull deepseek-r1:1.5b
```

### 3. 下载[本项目可执行文件](https://github.com/nfyxhan/deepseek-r1/releases/download/v0.0.0/deepseek-windows)

### 4. 启动本项目 (通过 win + R 打开cmd，输入以下命令)
```
chmod +x deepseek-windows
./deepseek-windows serve
```

### 4. 打开浏览器，访问 http://localhost:1203/
