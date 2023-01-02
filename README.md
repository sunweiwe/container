# container

它是一组提供容器假象的 Linux 操作系统原语。一个或一组进程可以摆脱它们的环境或名称空间，而运行在它们自己的新名称空间中，独立于主机的“默认”名称空间。像 Docker 这样的容器管理系统使得在您的机器上管理容器变得非常容易。但是这些容器是如何构造的呢?它只是一个 Linux 系统调用序列(主要涉及名称空间和 cgroup)，在非常基础的级别上，同时也利用了其他现有的 Linux 技术，用于容器文件系统、网络等。

## 为什么

本文的主要目的是让读者了解容器在 Linux 系统调用级是如何工作的。 container 允许您创建容器、管理容器映像、在现有容器中执行进程等。

## 能力

- 使用 Overlay 文件系统快速创建容器，而不需要复制整个文件系统，同时也在多个容器实例之间共享相同的容器映像。
- 容器拥有自己的网络名称空间，并且能够访问 internet。请参阅下面的限制。
- 你可以控制系统资源，如 CPU 百分比、RAM 数量和进程数量。通过利用 cgroups 实现了这一点。

虽然创建了 cgroups 来限制以下资源，但容器将使用无限的资源，除非在 'run'命令中指定'——mem '、'——cpus '或'——pid '选项。这些标志分别限制了容器可以消耗的最大 RAM、CPU 内核和 pid。

- Number of CPU cores
- RAM
- Number of PIDs (to limit processes)

## 容器隔离

- File system (via `chroot`)
- PID
- IPC
- UTS (hostname)
- Mount
- Network

## 清理 CGROUP

```bash
cgclear
```

## 限制

- 目前不支持暴露主机上的容器端口。每当 Docker 容器需要公开主机上的端口时，Docker 使用程序“Docker -proxy”作为代理来完成这一任务。 需要开发一个类似的代理。虽然现在容器可以访问 internet，但是能够公开主机上的端口将是一个很好的特性(主要是了解如何做到这一点)。
- 错误处理不好。如果出现错误，特别是在试图运行容器时，可能无法干净地卸载一些文件系统。

## 接入互联网

当你第一次运行容器时，一个新的桥'container0'被创建。因为所有的容器网络接口都连接到这个桥，所以它们可以相互通信，而不需要您做任何事情。为了使容器能够到达 internet，您需要在主机上启用数据包转发。为此，提供了一个方便的脚本' enable_internet.sh '。在运行它之前，您可能需要更改它以反映您的 internet 连接接口的名称。剧本里有说明。运行此程序后，容器应该能够连接到 internet 并安装包等。

```bash
sudo ./bin/container run alpine /bin/sh

ifconfig

ps aux

apk add python3

sudo ./bin/container ps
```
