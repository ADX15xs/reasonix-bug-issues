#!/bin/bash
# SenseNova 文生图 API 调用脚本
# 用法:
#   ./generate_image.sh                          # 使用默认 prompt 生成图片
#   ./generate_image.sh path/to/prompt.json      # 使用自定义 JSON 文件生成图片
#   ./generate_image.sh --prompt "你的prompt文本" # 直接传入 prompt 文本
#   ./generate_image.sh -o filename.jpg          # 指定输出文件名
#   ./generate_image.sh path/to/prompt.json -o reasonix-bug-dashboard-cn.jpg  # 组合使用

set -e

# 加载 .env 文件
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"

if [ -f "$ENV_FILE" ]; then
    while IFS='=' read -r key value || [ -n "$key" ]; do
        value="${value%$'\r'}"
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        key=$(echo "$key" | xargs)
        value=$(echo "$value" | xargs)
        export "$key=$value"
    done < "$ENV_FILE"
    echo "[OK] 已加载 .env 文件: $ENV_FILE"
else
    echo "[ERROR] 未找到 .env 文件: $ENV_FILE"
    exit 1
fi

# 检查 API Key
if [ -z "$SENSENOVA_API_KEY" ]; then
    echo "[ERROR] SENSENOVA_API_KEY 未设置，请在 .env 文件中配置"
    exit 1
fi
echo "[OK] SENSENOVA_API_KEY 已配置"

# API 配置
API_URL="https://token.sensenova.cn/v1/images/generations"
MODEL="sensenova-u1-fast"
SIZE="2752x1536"
N=1

# 默认 prompt（信息图生成）
DEFAULT_PROMPT_FILE=$(mktemp /tmp/sensenova_default_prompt_XXXXXX.txt)
cat > "$DEFAULT_PROMPT_FILE" << 'PROMPT_EOF'
这张信息图以柔和的粉色、淡黄色和浅蓝色为主色调，采用了极具亲和力的可爱卡通风格（包含猫咪、拟人化表情等元素）。整体排版从左到右分为三个主要区块，分别介绍核心能力、工作流程和重要规则。图表的左上角是醒目的主标题"信息图生成专家"，其下方紧跟副标题："帮助用户将复杂信息转化为清晰易懂的视觉呈现"。以下是图表中各区块的详细结构和全部文字内容：1. 左侧区块：核心能力与专家提示。该区块主要列出了三项核心能力，并附带了一条专家提示。核心能力（由上至下排列，每项均配有可爱的拟人化图标）：1. 联网搜索功能：查询最新网络信息（图标为一个带有猫耳、拿着放大镜的拟人化地球）。2. 网页内容读取功能：获取指定网页的详细内容（图标为一个戴着眼镜、正在阅读的拟人化纸卷）。3. 信息图生成功能：根据文字描述生成专业的信息图（图标为一个手持柱状图的可爱机器人）。专家提示（位于该区块底部，背景为黄色便利贴样式，右上角有一只探出纸箱的猫咪图标）：文本内容："熟练结合搜索与读取工具，最大化提升视觉数据的信息密度。"2. 中间区块：严格工作流程。该区块通过一条带有节点的垂直轴线，串联起三个工作步骤，每个步骤放置在圆角标签中：步骤一：分析需求（图标为粉色的大脑与一个带有笑脸的彩色齿轮）。步骤二：收集信息（图标为一个拟人化的漏斗，正在过滤星星和圆点）。步骤三：生成图片（图标为带有猫爪印的调色板、画笔以及一个饼状图）。3. 右侧区块：重要规则与核心目标。该区块被设计成一个带有红白相间遮阳篷的备忘录面板。重要规则（包含四条规则，每条规则左侧配有图标）：规则一：每次请求都是全新任务（图标为一个猫咪造型的甜甜圈）。规则二：优先使用辅助工具收集信息（图标为一个带有星星装饰的可爱手提包）。规则三：保留用户原始数据（图标为一个带有花朵旋钮的拟人化保险箱）。规则四：使用用户语言生成内容（图标为一块玉石质感的云纹装饰和一支铅笔）。核心目标（位于该区块底部，背景为淡紫色，右侧配有一个礼物盒图标）：文本内容："消除混乱，重构逻辑，实现高维维度视觉数据合成。"
PROMPT_EOF

# 解析参数
PROMPT_SOURCE=""
PROMPT_TEXT=""
OUTPUT_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --prompt)
            PROMPT_SOURCE="inline"
            PROMPT_TEXT="$2"
            shift 2
            ;;
        *)
            if [ -f "$1" ]; then
                PROMPT_SOURCE="file"
                JSON_FILE="$1"
            elif [ -z "$PROMPT_SOURCE" ]; then
                echo "[ERROR] 无效参数: $1"
                echo "用法: $0 [prompt_json_file | --prompt \"prompt文本\"] [-o output_filename]"
                exit 1
            fi
            shift
            ;;
    esac
done

if [ -z "$PROMPT_SOURCE" ]; then
    PROMPT_SOURCE="default"
fi

echo ""
echo "=========================================="
echo "  SenseNova 文生图 API 调用"
echo "=========================================="
echo "模型: $MODEL"
echo "尺寸: $SIZE"
echo "数量: $N"
echo "=========================================="
echo ""

# 创建 JSON payload
TMPJSON=$(mktemp /tmp/sensenova_payload_XXXXXX.json)

if [ "$PROMPT_SOURCE" = "file" ]; then
    # 直接使用传入的 JSON 文件
    cp "$JSON_FILE" "$TMPJSON"
    echo "[INFO] 使用自定义 JSON 文件: $JSON_FILE"
elif [ "$PROMPT_SOURCE" = "inline" ]; then
    # 从命令行参数构建 JSON
    python3 -c "
import json, sys
payload = {
    'model': '$MODEL',
    'prompt': '''$PROMPT_TEXT''',
    'size': '$SIZE',
    'n': $N
}
with open('$TMPJSON', 'w', encoding='utf-8') as f:
    json.dump(payload, f, ensure_ascii=False, indent=2)
print(f'[INFO] Prompt 长度: {len(payload[\"prompt\"])} 字符')
"
else
    # 使用默认 prompt 构建 JSON
    python3 -c "
import json
with open('$DEFAULT_PROMPT_FILE', 'r', encoding='utf-8') as f:
    prompt = f.read().strip()
payload = {
    'model': '$MODEL',
    'prompt': prompt,
    'size': '$SIZE',
    'n': $N
}
with open('$TMPJSON', 'w', encoding='utf-8') as f:
    json.dump(payload, f, ensure_ascii=False, indent=2)
print(f'[INFO] Prompt 长度: {len(prompt)} 字符')
"
    rm -f "$DEFAULT_PROMPT_FILE"
fi

echo "[INFO] 正在调用 API..."
echo ""

# 调用 API
HTTP_CODE=$(curl -s -w "%{http_code}" -o /tmp/sensenova_response.json \
  -X POST "$API_URL" \
  -H "Authorization: Bearer $SENSENOVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d @"$TMPJSON")

# 清理临时 JSON 文件
rm -f "$TMPJSON"

echo "[INFO] HTTP 状态码: $HTTP_CODE"
echo ""

# 检查响应
if [ "$HTTP_CODE" = "200" ]; then
    echo "=========================================="
    echo "  ✅ API 调用成功！"
    echo "=========================================="
    echo ""
    echo "响应内容："
    cat /tmp/sensenova_response.json
    echo ""
    echo ""
    
    # 尝试提取图片 URL
    IMAGE_URL=$(python3 -c "
import json
with open('/tmp/sensenova_response.json') as f:
    data = json.load(f)
if 'data' in data and len(data['data']) > 0:
    item = data['data'][0]
    if 'url' in item:
        print(item['url'])
    elif 'b64_json' in item:
        print('base64_encoded')
" 2>/dev/null || echo "")
    
    if [ -n "$IMAGE_URL" ] && [ "$IMAGE_URL" != "base64_encoded" ]; then
        echo "图片 URL: $IMAGE_URL"
        echo ""
        echo "[INFO] 正在下载图片..."
        if [ -z "$OUTPUT_FILE" ]; then
            OUTPUT_FILE="generated_image_$(date +%Y%m%d_%H%M%S).png"
        fi
        curl -s -o "$OUTPUT_FILE" "$IMAGE_URL"
        if [ -f "$OUTPUT_FILE" ]; then
            echo "[OK] 图片已保存: $OUTPUT_FILE"
        fi
    elif [ "$IMAGE_URL" = "base64_encoded" ]; then
        echo "[INFO] 响应为 base64 编码，正在解码..."
        if [ -z "$OUTPUT_FILE" ]; then
            OUTPUT_FILE="generated_image_$(date +%Y%m%d_%H%M%S).png"
        fi
        python3 -c "
import json, base64
with open('/tmp/sensenova_response.json') as f:
    data = json.load(f)
b64 = data['data'][0]['b64_json']
with open('$OUTPUT_FILE', 'wb') as out:
    out.write(base64.b64decode(b64))
print('[OK] 图片已保存: $OUTPUT_FILE')
"
    fi
else
    echo "=========================================="
    echo "  ❌ API 调用失败"
    echo "=========================================="
    echo ""
    echo "HTTP 状态码: $HTTP_CODE"
    echo "错误响应:"
    cat /tmp/sensenova_response.json
    echo ""
    exit 1
fi

# 清理
rm -f /tmp/sensenova_response.json
