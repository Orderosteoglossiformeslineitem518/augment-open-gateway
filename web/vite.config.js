import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { resolve } from 'path'
import obfuscator from 'vite-plugin-javascript-obfuscator'

export default defineConfig(({ mode }) => {
  const isProduction = mode === 'production'

  const plugins = [
    vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
      imports: ['vue', 'vue-router', 'pinia'],
      dts: true
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: true
    })
  ]

  // 只在生产环境启用代码混淆（降低强度配置）
  if (isProduction) {
    plugins.push(
      obfuscator({
        options: {
          // 基础混淆配置 - 降低强度
          compact: false, // 不压缩代码，保持可读性
          controlFlowFlattening: false, // 禁用控制流扁平化，避免执行问题
          deadCodeInjection: false,
          debugProtection: false,
          debugProtectionInterval: 0,
          disableConsoleOutput: false, // 保留console，便于调试
          identifierNamesGenerator: 'hexadecimal',
          log: false,
          numbersToExpressions: false, // 禁用数字表达式转换
          renameGlobals: false,
          selfDefending: false,
          simplify: true,
          splitStrings: false, // 禁用字符串分割
          stringArray: false, // 禁用字符串数组混淆，避免模块路径解析问题
          stringArrayCallsTransform: false, // 禁用字符串数组调用转换
          stringArrayEncoding: [], // 不使用编码，避免解析问题
          stringArrayIndexShift: false,
          stringArrayRotate: false,
          stringArrayShuffle: false,
          stringArrayWrappersCount: 1, // 减少包装器数量
          stringArrayWrappersChainedCalls: false,
          stringArrayWrappersParametersMaxCount: 2,
          stringArrayWrappersType: 'variable', // 使用变量而非函数
          stringArrayThreshold: 0.5, // 降低字符串数组阈值
          transformObjectKeys: false, // 禁用对象键转换
          unicodeEscapeSequence: false
        },
        // 排除 node_modules，避免第三方库混淆问题
        exclude: [/node_modules/]
      })
    )
  }

  return {
    plugins,
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src')
      }
    },
    server: {
      port: 23000,
      proxy: {
        '/api': {
          target: 'http://localhost:28080',
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: 'dist',
      assetsDir: 'assets',
      // 生产环境完全禁用 sourcemap
      sourcemap: false,
      // 启用温和压缩
      minify: 'terser',
      terserOptions: {
        compress: {
          // 保留 console，便于调试
          drop_console: false,
          // 移除 debugger
          drop_debugger: true,
          // 移除无用代码
          dead_code: true,
          // 禁用一些可能导致问题的压缩选项
          sequences: false, // 禁用序列优化
          booleans: false, // 禁用布尔值优化
          loops: false, // 禁用循环优化
          unused: false, // 禁用未使用变量移除
          conditionals: false, // 禁用条件语句优化
          evaluate: false // 禁用表达式求值
        },
        mangle: {
          // 温和的变量名混淆
          toplevel: false, // 不混淆顶级作用域
          keep_fnames: true, // 保留函数名
          reserved: ['$', 'exports', 'require'] // 保留关键字
        },
        format: {
          // 保持一定的代码格式
          beautify: false,
          comments: false
        }
      },
      rollupOptions: {
        output: {
          // 文件名随机化
          entryFileNames: 'assets/[name]-[hash].js',
          chunkFileNames: 'assets/[name]-[hash].js',
          assetFileNames: 'assets/[name]-[hash].[ext]',
          manualChunks: {
            vendor: ['vue', 'vue-router', 'pinia'],
            element: ['element-plus'],
            charts: ['echarts', 'vue-echarts']
          }
        }
      }
    }
  }
})
