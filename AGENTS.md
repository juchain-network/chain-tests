# chain-tests AGENTS Guide

## 1. 项目目标

`chain-tests` 的目标是建设 **自定义公链（Congress 共识）本地集成测试能力**，重点验证：

- 系统合约在真实多节点链环境下的端到端行为；
- 共识层与系统合约之间的联动行为；
- 共识关键路径（验证人集合更新、惩罚、奖励、治理参数变更）的回归稳定性。
- 支持 `docker` 与 `native`（非 Docker）两种多节点运行后端，并可通过配置切换。

该仓库是集成测试工程主仓，优先承载测试编排、测试用例、报告与测试资产，不承载系统合约主开发。

---

## 2. 关联仓库与职责边界

### 2.1 系统合约仓库

- 路径：`/Users/litian/code/work/github/chain-contract`
- 职责：系统合约源码、Foundry 单测、Genesis 相关脚本、Go 合约绑定生成。

### 2.2 现有集成测试雏形（迁移参考）

- 路径：`/Users/litian/code/work/github/chain-contract/test-integration`
- 职责：当前可运行的本地 4 节点集成测试框架雏形（Makefile、Docker、Go tests、CI runner）。
- 说明：`chain-tests` 新能力建设应优先复用其设计思路与测试分组方式，再逐步抽离为独立测试工程。

### 2.3 定制 geth 仓库

- 参考路径：`/Users/litian/code/work/github/chain/accounts`
- 实际仓库根目录：`/Users/litian/code/work/github/chain`
- 职责：节点实现、共识执行、系统交易保护逻辑。
- 依赖边界：`chain-tests` 仅消费 `chain` 编译产出的二进制，不依赖 `chain/local-test` 脚本与配置。

### 2.4 Congress 共识实现

- 核心文件：`/Users/litian/code/work/github/chain/consensus/congress/congress.go`
- 当前关键常量（实现侧）：
  - 默认 `epochLength = 86400`
  - `maxValidators = 21`
  - 系统合约地址：
    - Validators: `0x...f010`
    - Punish: `0x...f011`
    - Proposal: `0x...f012`
    - Staking: `0x...f013`

### 2.5 chain-contract 依赖边界（新增）

- `chain-tests` 仅依赖 `chain-contract` 的编译产物（如 `out/` 下 artifact / bytecode）。
- 本仓库不负责在 `chain-contract` 仓库内执行源码构建；需要提前准备好编译结果并通过配置路径引用。

---

## 3. 推荐目录设计（chain-tests）

建议保持与 `test-integration` 一致的可迁移结构：

```text
chain-tests/
├── AGENTS.md
├── Makefile
├── ci.go
├── docker/
├── scripts/
├── internal/
│   ├── context/
│   └── config/
├── tests/
├── templates/
├── data/                 # 运行态生成，不入库
└── reports/              # 测试报告输出
```

---

## 4. 测试执行基线（建议）

### 4.0 运行时后端选择（新增）

推荐采用双后端设计：

- `native`（默认推荐，本地开发优先）：
  - 使用 `pm2` 管理多 `geth` 进程；
  - 使用本仓库维护的 `scripts/native/pm2_init.sh` 与 `native/ecosystem.config.js`；
  - 优势：启动/停止更快、无镜像构建、调试日志更直接。
- `docker`（CI 与环境一致性优先）：
  - 继续使用 `docker compose`；
  - 优势：依赖隔离强、跨机器一致性好。

运行时后端建议通过统一配置文件 `config/test_env.yaml` 的 `runtime.backend` 字段选择：

- `runtime.backend: native` -> 调用本地进程编排脚本
- `runtime.backend: docker` -> 调用 docker 编排脚本

依赖路径建议通过 `config/test_env.yaml` 的 `paths.*` 管理，支持相对路径（相对本仓库根目录），例如：

- `paths.chain_root: ../chain`
- `paths.chain_contract_root: ../chain-contract`
- `paths.chain_contract_out: ../chain-contract/out`

Epoch 建议通过 `config/test_env.yaml` 的 `network.epoch` 配置（如 `30` / `60`），并在生成 `genesis.json` 时生效。
也可在初始化时临时覆盖：`make init EPOCH=60`（仅影响本次生成）。
建议通过 `tests.profile`（`fast/default/edge`）统一管理测试参数窗口（cooldown/lasting/unbonding 等），减少散落硬编码。
高频轮询等待建议通过 `data/test_config.yaml` 的 `test.timing.retry_poll_ms` / `test.timing.block_poll_ms` 调整，优先在配置层调优而非改代码常量。

### 4.1 本地网络编排（4 节点）

- 使用 Docker Compose 启动 4 节点（3 验证节点 + 1 同步节点）。
- 对外 RPC 统一走 `http://localhost:18545`。
- 启动前自动生成：
  - 节点密钥与配置；
  - `genesis.json`（含系统合约 alloc）；
  - `data/test_config.yaml`（测试账户与 RPC 配置）。

### 4.2 建议命令约定

- 初始化并启动：
  - `make reset`（等价 clean + init + run + ready）
- 运行全部测试：
  - `make test-all`
  - `make ci-groups-budget`（按分组执行并启用默认耗时预算门禁）
  - `make ci-tests-budget RUN='TestI_PublicQueryCoverage' PKGS=./tests/rewards`（按用例模式执行并启用慢用例预算门禁）
  - `make ci-budget-suggest`（基于历史 reports 自动推荐预算阈值）
  - `make ci-budget-suggest-save`（把推荐阈值写入 `config/ci_budget.local.mk` 本地覆盖文件）
- 分组运行：
  - `make test-config`
  - `make test-governance`
  - `make test-staking`
  - `make test-delegation`
  - `make test-punish`
  - `make test-rewards`
  - `make test-epoch`
- 查看日志：
  - `make logs`

建议将命令入口统一为：

- `make net-up`
- `make net-down`
- `make net-reset`
- `make net-ready`

以上命令内部根据 `runtime.backend` 自动路由到 `native` 或 `docker` 实现，测试命令无需关心底层后端。

---

## 5. 共识/合约联调硬约束（必须遵守）

1. Epoch 生效延迟  
验证人集合变化通常在 Epoch 边界生效；新增/移除验证人相关断言必须等待到下一个 Epoch 检查。

2. 物理节点数量约束  
本地仅 4 物理节点时，不要让“活跃验证人阈值”超过网络可达成共识的上限，否则可能停链。

3. 提案 -> 投票 -> 注册 的准入顺序  
候选验证人必须先提案通过，再在有效窗口内注册；跳步会失败。

4. 系统交易保护  
`distributeBlockReward`、`punish`、`updateActiveValidatorSet` 等系统级方法不可通过普通外部交易直接调用；集成测试要验证副作用，而不是强行直调。

5. 交易参数  
优先使用 Legacy gas（`GasPrice`）路径，避免在本地链上出现不兼容的 EIP-1559 行为差异。

6. Nonce 并发安全  
并发发交易时统一走上下文封装（如 `CIContext` 风格）管理 nonce，避免 flaky。

---

## 6. 变更联动规则

当系统合约变更时，集成测试侧必须同步完成以下步骤：

1. 在 `chain-contract` 编译合约：`forge build`
2. 生成/更新 Go 合约绑定
3. 重新生成系统合约 bytecode 与 `genesis.json`
4. 重置本地测试链数据并重启网络
5. 重新执行受影响测试分组

当 Congress 共识逻辑变更（`congress.go`）时，必须：

1. 重新构建定制 geth 二进制
2. 替换测试网络运行二进制
3. 从干净数据目录重启后再跑回归

---

## 7. 测试开发规范

1. 用例命名  
建议使用稳定前缀分组（如 `TestA_`, `TestB_`），便于 CI 分组执行与故障定位。

2. 断言策略  
先断言链状态（高度推进、交易上链），再断言业务状态（validator set、stake、proposal、rewards）。

3. 隔离性  
高耦合场景（惩罚、退出、epoch 切换）建议单测单起链或按组重置网络，避免级联污染。

4. 可观测性  
失败日志必须包含：测试名、交易哈希、当前块高、关键配置参数快照。

5. 禁止事项  
- 不依赖手工操作修复状态；
- 不在脏数据目录上做“重试即通过”的不稳定测试；
- 不绕过共识约束构造“主网不会发生”的路径作为成功标准。

---

## 8. 当前阶段工作重点

1. 将 `chain-contract/test-integration` 的稳定能力迁移到 `chain-tests`（先跑通，再重构）。
2. 优先补齐 Congress 相关高风险回归场景：
   - Epoch 边界验证人更新；
   - 惩罚与恢复；
   - 治理参数动态调整对共识行为的影响；
   - 升级/初始化保护路径。
3. 建立可重复、可追溯的本地 CI 报告输出。
4. 新增双后端运行时：
   - `native(pm2)` 用于开发快速反馈；
   - `docker` 用于 CI 稳定回归；
   - 保证两者复用同一套测试用例与断言逻辑。

---

## 9. 双后端落地建议（实施顺序）

1. 定义统一配置：`config/test_env.yaml`（见 `config/test_env.yaml.example`）。
2. 新增统一网络编排入口（建议 `scripts/network/*.sh`）：
   - `docker.sh`：封装 compose 的 up/down/logs/ready；
   - `native.sh`：封装 pm2 的 init/start/stop/logs/ready；
   - `dispatch.sh`：读取配置并分发到对应后端。
3. `Makefile` 仅调用 `dispatch.sh`，不直接依赖具体后端命令。
4. CI 默认强制 `runtime.backend=docker`，本地默认 `runtime.backend=native`。

当前迁移阶段说明：

- `native` 后端已支持节点启停与就绪检查（`pm2` 进程编排）。
- 集成测试数据源（`data/test_config.yaml`）目前仍以 `gen_network_config.sh` 生成流程为主，与 `native` 本地链配置尚未完全统一。
- 在账户与创世配置统一前，建议：
  - 本地链调试优先 `native`；
  - 自动化回归优先 `docker`。
