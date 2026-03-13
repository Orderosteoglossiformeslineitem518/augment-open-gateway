/**
 * Lobe Icons 图标配置
 * 使用 CDN 加载彩色 WEBP 图标
 * 文档: https://github.com/lobehub/lobe-icons
 */

// CDN 基础 URL（使用阿里云镜像，国内访问更快）- 彩色 WEBP 版本
export const LOBE_ICONS_CDN_BASE = 'https://registry.npmmirror.com/@lobehub/icons-static-webp/latest/files'

// 备用 CDN（unpkg）
export const LOBE_ICONS_CDN_BACKUP = 'https://unpkg.com/@lobehub/icons-static-webp@latest'

/**
 * 获取图标 URL（彩色 WEBP）
 * @param {string} iconName - 图标名称，如 'openai', 'claude'
 * @param {'light' | 'dark'} theme - 主题模式，默认 'light'
 * @param {boolean} useBackup - 是否使用备用 CDN
 * @returns {string} 图标 URL
 */
export const getIconUrl = (iconName, theme = 'light', useBackup = false) => {
  if (!iconName) return ''
  const base = useBackup ? LOBE_ICONS_CDN_BACKUP : LOBE_ICONS_CDN_BASE
  return `${base}/${theme}/${iconName}.webp`
}

/**
 * 可用的 AI/LLM 图标列表
 * 按品牌分类，便于选择
 */
export const ICON_CATEGORIES = [
  {
    category: '热门模型',
    icons: [
      { name: 'claude', label: 'Claude' },
      { name: 'openai', label: 'OpenAI' },
      { name: 'gemini', label: 'Gemini' },
      { name: 'deepseek', label: 'DeepSeek' },
      { name: 'mistral', label: 'Mistral' },
      { name: 'grok', label: 'Grok' },
    ]
  },
  {
    category: '国内模型',
    icons: [
      { name: 'qwen', label: '通义千问' },
      { name: 'doubao', label: '豆包' },
      { name: 'kimi', label: 'Kimi' },
      { name: 'zhipu', label: '智谱' },
      { name: 'baichuan', label: '百川' },
      { name: 'minimax', label: 'Minimax' },
      { name: 'hunyuan', label: '腾讯混元' },
      { name: 'spark', label: '讯飞星火' },
      { name: 'wenxin', label: '文心一言' },
      { name: 'yi', label: '零一万物' },
      { name: 'stepfun', label: '阶跃星辰' },
      { name: 'sensenova', label: '商汤日日新' },
      { name: 'skywork', label: '天工' },
    ]
  },
  {
    category: '云服务商',
    icons: [
      { name: 'anthropic', label: 'Anthropic' },
      { name: 'google', label: 'Google' },
      { name: 'azure', label: 'Azure' },
      { name: 'aws', label: 'AWS' },
      { name: 'bedrock', label: 'Bedrock' },
      { name: 'vertexai', label: 'VertexAI' },
      { name: 'alibabacloud', label: '阿里云' },
      { name: 'tencentcloud', label: '腾讯云' },
      { name: 'huaweicloud', label: '华为云' },
      { name: 'volcengine', label: '火山引擎' },
      { name: 'siliconcloud', label: 'SiliconCloud' },
    ]
  },
  {
    category: 'API 平台',
    icons: [
      { name: 'openrouter', label: 'OpenRouter' },
      { name: 'together', label: 'Together' },
      { name: 'groq', label: 'Groq' },
      { name: 'deepinfra', label: 'DeepInfra' },
      { name: 'fireworks', label: 'Fireworks' },
      { name: 'replicate', label: 'Replicate' },
      { name: 'perplexity', label: 'Perplexity' },
      { name: 'cohere', label: 'Cohere' },
      { name: 'sambanova', label: 'SambaNova' },
      { name: 'cerebras', label: 'Cerebras' },
      { name: 'novita', label: 'Novita' },
    ]
  },
  {
    category: '其他',
    icons: [
      { name: 'ollama', label: 'Ollama' },
      { name: 'lmstudio', label: 'LM Studio' },
      { name: 'huggingface', label: 'HuggingFace' },
      { name: 'nvidia', label: 'Nvidia' },
      { name: 'meta', label: 'Meta' },
      { name: 'github', label: 'GitHub' },
      { name: 'cursor', label: 'Cursor' },
      { name: 'windsurf', label: 'Windsurf' },
    ]
  }
]

/**
 * 获取所有图标的扁平列表
 * @returns {Array<{name: string, label: string}>}
 */
export const getAllIcons = () => {
  const icons = []
  ICON_CATEGORIES.forEach(cat => {
    cat.icons.forEach(icon => {
      icons.push(icon)
    })
  })
  return icons
}

/**
 * 根据图标名称获取显示标签
 * @param {string} iconName - 图标名称
 * @returns {string} 显示标签
 */
export const getIconLabel = (iconName) => {
  const allIcons = getAllIcons()
  const icon = allIcons.find(i => i.name === iconName)
  return icon ? icon.label : iconName
}

