FROM golang:1.24-bullseye

# 使用阿里云镜像源加速（解决国内访问 Debian 官方源慢的问题）
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources 2>/dev/null || \
    (echo "deb http://mirrors.aliyun.com/debian/ bullseye main" > /etc/apt/sources.list && \
     echo "deb http://mirrors.aliyun.com/debian/ bullseye-updates main" >> /etc/apt/sources.list && \
     echo "deb http://mirrors.aliyun.com/debian-security bullseye-security main" >> /etc/apt/sources.list)

# 安装时区数据并设置为东八区（中国时区）
RUN apt-get update -o Acquire::http::Timeout=30 && \
    apt-get install -y --no-install-recommends tzdata && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# 设置时区环境变量（东八区）
ENV TZ=Asia/Shanghai

WORKDIR /app

CMD ["sleep", "infinity"]


