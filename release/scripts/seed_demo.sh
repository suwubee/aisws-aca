#!/bin/bash
# 演示数据填充脚本
# 用法: ./seed_demo.sh

set -e

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
JSON_DIR="$ROOT/json"
API_BASE="http://localhost:34007/api"

echo "=== 演示数据填充脚本 ==="

# 登录获取token
echo "登录中..."
LOGIN_RESP=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')

TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "登录失败: $LOGIN_RESP"
  exit 1
fi
echo "登录成功"

# 创建本地服务器
echo "创建服务器..."
SERVER_RESP=$(curl -s -X POST "$API_BASE/servers" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"本地开发服务器","host":"127.0.0.1","port":22,"username":"root","auth_type":"password","password":"demo123"}')
SERVER_ID=$(echo "$SERVER_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$SERVER_ID" ]; then
  echo "  + 本地开发服务器"
else
  echo "  ! 服务器创建失败，尝试获取已有服务器"
  SERVER_ID=$(curl -s -H "Authorization: Bearer $TOKEN" "$API_BASE/servers" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
fi

# 填充项目
echo "填充项目数据..."
PROJECT_IDS=()
while IFS= read -r project; do
  RESP=$(curl -s -X POST "$API_BASE/projects" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "$project")
  NAME=$(echo "$project" | grep -o '"name":"[^"]*"' | cut -d'"' -f4)
  ID=$(echo "$RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ -n "$ID" ]; then
    echo "  + $NAME"
    PROJECT_IDS+=("$ID")
  else
    echo "  ! 创建失败: $NAME"
  fi
done < <(cat "$JSON_DIR/projects.json" | python3 -c "import sys,json; [print(json.dumps(p)) for p in json.load(sys.stdin)]")

# 填充工作流
echo "填充工作流数据..."
IDX=0
while IFS= read -r workflow; do
  # 关联到对应项目
  if [ $IDX -lt ${#PROJECT_IDS[@]} ]; then
    PID="${PROJECT_IDS[$IDX]}"
    workflow=$(echo "$workflow" | python3 -c "import sys,json; w=json.load(sys.stdin); w['project_id']='$PID'; print(json.dumps(w))")
  fi

  RESP=$(curl -s -X POST "$API_BASE/workflows" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "$workflow")
  NAME=$(echo "$workflow" | grep -o '"name":"[^"]*"' | cut -d'"' -f4)
  ID=$(echo "$RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ -n "$ID" ]; then
    echo "  + $NAME"
  else
    echo "  ! 创建失败: $NAME"
  fi
  IDX=$((IDX + 1))
done < <(cat "$JSON_DIR/workflows.json" | python3 -c "import sys,json; [print(json.dumps(w)) for w in json.load(sys.stdin)]")

# 填充任务
echo "填充任务数据..."
if [ -f "$JSON_DIR/tasks.json" ]; then
  IDX=0
  while IFS= read -r task; do
    # 关联到对应项目和服务器
    if [ $IDX -lt ${#PROJECT_IDS[@]} ]; then
      PID="${PROJECT_IDS[$((IDX % ${#PROJECT_IDS[@]}))]}"
      task=$(echo "$task" | python3 -c "import sys,json; t=json.load(sys.stdin); t['project_id']='$PID'; t['server_id']='$SERVER_ID'; print(json.dumps(t))")
    fi

    RESP=$(curl -s -X POST "$API_BASE/tasks" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "$task")
    TITLE=$(echo "$task" | python3 -c "import sys,json; print(json.load(sys.stdin).get('title',''))")
    ID=$(echo "$RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -n "$ID" ]; then
      echo "  + $TITLE"
    else
      echo "  ! 创建失败: $TITLE"
    fi
    IDX=$((IDX + 1))
  done < <(cat "$JSON_DIR/tasks.json" | python3 -c "import sys,json; [print(json.dumps(t)) for t in json.load(sys.stdin)]")
fi

echo "=== 填充完成 ==="
