import { useEffect, useRef, useState } from 'react'
import { Button, Input, Empty, message } from 'antd'
import {
  PlayCircleOutlined,
  StopOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined,
  ClearOutlined,
  MinusCircleOutlined,
} from '@ant-design/icons'
import { useStore } from '../../store'
import './RunPanel.css'

const { TextArea } = Input

const DEFAULT_INPUTS_TEXT = '{\n  "question": "你好"\n}'

const buildInputsText = (nodes: Array<{ type?: string; data?: any }>) => {
  const inputTemplate: Record<string, unknown> = {}

  for (const node of nodes) {
    if (node.type === 'start' && Array.isArray(node.data?.variables)) {
      for (const variable of node.data.variables) {
        if (variable?.key) {
          inputTemplate[variable.key] = variable.value ?? ''
        }
      }
    }

    if (node.type === 'userInput' && node.data?.inputField) {
      const field = node.data.inputField
      if (!(field in inputTemplate)) {
        inputTemplate[field] = ''
      }
    }
  }

  if (Object.keys(inputTemplate).length === 0) {
    return DEFAULT_INPUTS_TEXT
  }

  return JSON.stringify(inputTemplate, null, 2)
}

const RunPanel: React.FC = () => {
  const {
    currentWorkflow,
    nodes,
    edges,
    executionStates,
    executionStatus,
    saveWorkflow,
    streamRunWorkflow,
    setExecutionStatus,
    clearExecutionStates,
  } = useStore()

  const [inputsText, setInputsText] = useState(DEFAULT_INPUTS_TEXT)
  const [isRunning, setIsRunning] = useState(false)
  const preparedWorkflowIdRef = useRef<string | null>(null)

  useEffect(() => {
    const workflowId = currentWorkflow?.id ?? null
    if (!workflowId) {
      preparedWorkflowIdRef.current = null
      setInputsText(DEFAULT_INPUTS_TEXT)
      return
    }

    if (preparedWorkflowIdRef.current !== workflowId) {
      setInputsText(buildInputsText(nodes))
      preparedWorkflowIdRef.current = workflowId
    }
  }, [currentWorkflow?.id, nodes])

  const handleRun = async () => {
    const workflowId = currentWorkflow?.id
    if (!workflowId) {
      message.warning('请先选择工作流')
      return
    }

    let inputs: Record<string, any>
    try {
      const parsed = JSON.parse(inputsText)
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
        throw new Error('Inputs must be an object')
      }
      inputs = parsed as Record<string, any>
    } catch {
      message.error('输入参数必须是合法的 JSON 对象')
      return
    }

    if (nodes.length === 0) {
      message.warning('请先添加至少一个节点再运行工作流')
      return
    }

    setIsRunning(true)
    try {
      await saveWorkflow(workflowId, { nodes, edges })
      await streamRunWorkflow(workflowId, inputs)
    } catch (error) {
      message.error(error instanceof Error ? error.message : '工作流执行失败')
    } finally {
      setIsRunning(false)
    }
  }

  const handleStop = () => {
    setIsRunning(false)
    setExecutionStatus('stopped')
  }

  const handleClear = () => {
    clearExecutionStates()
    setExecutionStatus(null)
  }

  const statusIcon = (status: string) => {
    switch (status) {
      case 'running':
        return <LoadingOutlined spin style={{ color: 'var(--c-blue)' }} />
      case 'success':
        return <CheckCircleOutlined style={{ color: 'var(--c-green)' }} />
      case 'failed':
        return <CloseCircleOutlined style={{ color: 'var(--c-red)' }} />
      case 'skipped':
        return <MinusCircleOutlined style={{ color: 'var(--c-text-tertiary)' }} />
      default:
        return <ClockCircleOutlined style={{ color: 'var(--c-text-tertiary)' }} />
    }
  }

  const executedNodes = Object.values(executionStates)
  const hasResults = executedNodes.length > 0

  return (
    <div className="run-panel">
      <div className="run-panel-header">
        <h3>调试运行</h3>
      </div>

      <div className="run-panel-body">
        <div className="run-section">
          <label className="run-section-label">输入参数 (JSON)</label>
          <TextArea
            value={inputsText}
            onChange={(e) => setInputsText(e.target.value)}
            placeholder='{"question": "你好"}'
            rows={4}
            className="run-input-textarea"
            disabled={isRunning}
          />
        </div>

        <div className="run-actions">
          {isRunning ? (
            <Button
              danger
              icon={<StopOutlined />}
              onClick={handleStop}
              block
              size="middle"
            >
              停止
            </Button>
          ) : (
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={handleRun}
              block
              size="middle"
            >
              运行工作流
            </Button>
          )}
          {hasResults && !isRunning && (
            <Button
              icon={<ClearOutlined />}
              onClick={handleClear}
              size="middle"
              className="run-clear-btn"
            >
              清除
            </Button>
          )}
        </div>

        {executionStatus && (
          <div className={`run-status run-status--${executionStatus}`}>
            {statusIcon(executionStatus)}
            <span>
              {executionStatus === 'running'
                ? '运行中...'
                : executionStatus === 'success'
                ? '执行完成'
                : executionStatus === 'failed'
                ? '执行失败'
                : '已停止'}
            </span>
          </div>
        )}

        {hasResults ? (
          <div className="run-results">
            <label className="run-section-label">节点执行结果</label>
            {executedNodes.map((exec) => {
              const node = nodes.find((n) => n.id === exec.nodeId)
              return (
                <div key={exec.nodeId} className="run-result-card">
                  <div className="run-result-header">
                    {statusIcon(exec.status)}
                    <span className="run-result-name">
                      {(node?.data as any)?.label || exec.nodeId}
                    </span>
                    <span className="run-result-type">{node?.type}</span>
                  </div>
                  {(exec as any).output && (
                    <pre className="run-result-output">
                      {typeof (exec as any).output === 'string'
                        ? (exec as any).output
                        : JSON.stringify((exec as any).output, null, 2)}
                    </pre>
                  )}
                  {exec.error && (
                    <div className="run-result-error">{exec.error}</div>
                  )}
                </div>
              )
            })}
          </div>
        ) : (
          !isRunning && (
            <div className="run-empty">
              <Empty
                description="点击“运行工作流”开始调试"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            </div>
          )
        )}
      </div>
    </div>
  )
}

export default RunPanel
