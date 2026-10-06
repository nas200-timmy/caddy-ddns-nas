#!/usr/bin/env python3
"""冒烟测试：在 pty 上启动 cddns setup，等终端初始化完成后逐键输入。

使用假凭证部署：保存配置会成功，但"校验 DNS 凭证"必然失败——真实网络环境下
预期到达失败页。断言：配置已落盘（第一步保存配置），Ctrl+C 正常退出（退出码 0）。
"""
import os
import pty
import select
import sys
import time

BIN = "./bin/cddns"
CFG = "/tmp/cddns-smoke.yaml"
KEYS = b"\r127.0.0.1\r\r22\r\rAKID1234567890abcdef\rSK1234567890abcdef\rnas.example.com\r\r"
FAILED = "失败".encode()
DONE = "完成".encode()

pid, fd = pty.fork()
if pid == 0:
    os.execv(BIN, [BIN, "setup", "--config", CFG])
    os._exit(127)

# 等程序完成终端初始化（alt screen 等）后再注入按键
time.sleep(2.0)
os.write(fd, KEYS)

out = b""
sent_interrupt = False
status = None
deadline = time.time() + 60
while time.time() < deadline:
    r, _, _ = select.select([fd], [], [], 0.3)
    if r:
        try:
            data = os.read(fd, 8192)
            if data:
                out += data
        except OSError:
            # 子进程退出后 pty 读端会返回 EIO，属正常现象
            pass
    # 部署失败页出现后发送 Ctrl+C 退出；完成页会自动退出
    if not sent_interrupt and FAILED in out:
        sent_interrupt = True
        try:
            os.write(fd, b"\x03")
        except OSError:
            pass
    wpid, st = os.waitpid(pid, os.WNOHANG)
    if wpid == pid:
        status = st
        break

# 读完剩余输出（子进程已退出时读端可能直接 EIO，忽略）
while True:
    r, _, _ = select.select([fd], [], [], 0.2)
    if not r:
        break
    try:
        data = os.read(fd, 8192)
        if not data:
            break
        out += data
    except OSError:
        break

if status is None:
    # 尝试阻塞收尸，排除"子进程早已退出但没被 WNOHANG 捞到"的情况
    try:
        wpid, st = os.waitpid(pid, 0)
        if wpid == pid:
            status = st
    except ChildProcessError:
        pass

with open("/tmp/smoke.out", "wb") as f:
    f.write(out)

if status is None:
    print("TIMEOUT: 向导未在 60 秒内退出")
    try:
        os.kill(pid, 9)
    except ProcessLookupError:
        pass
    sys.exit(1)

if not os.WIFEXITED(status) or os.WEXITSTATUS(status) != 0:
    print(f"向导异常退出: status={status}")
    sys.exit(1)

if not os.path.exists(CFG):
    print("向导退出正常但配置文件未生成")
    sys.exit(1)

if FAILED not in out and DONE not in out:
    print("输出中既没有失败页也没有完成页标记，向导可能未走完流程")
    sys.exit(1)

print(f"OK: 向导正常退出（输出 {len(out)} 字节），配置已落盘")
