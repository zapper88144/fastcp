import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { ArrowLeft, Loader2, Globe, Zap } from 'lucide-react'
import { api } from '@/lib/api'
import type { PHPInstance } from '@/types'

export function CreateSitePage() {
  const navigate = useNavigate()
  const [isLoading, setIsLoading] = useState(false)
  const [phpVersions, setPHPVersions] = useState<PHPInstance[]>([])
  const [error, setError] = useState('')

  const [form, setForm] = useState({
    name: '',
    domain: '',
    php_version: '8.4',
    public_path: 'public',
    worker_mode: false,
    worker_file: 'index.php',
    worker_num: 2,
  })

  useEffect(() => {
    async function fetchPHPVersions() {
      try {
        const data = await api.getPHPInstances()
        setPHPVersions(data.instances || [])
        if (data.instances?.length > 0) {
          setForm((f) => ({ ...f, php_version: data.instances[0].version }))
        }
      } catch (error) {
        console.error('Failed to fetch PHP versions:', error)
      }
    }
    fetchPHPVersions()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      const site = await api.createSite({
        name: form.name || form.domain,
        domain: form.domain,
        php_version: form.php_version,
        public_path: form.public_path,
        worker_mode: form.worker_mode,
        worker_file: form.worker_mode ? form.worker_file : undefined,
        worker_num: form.worker_mode ? form.worker_num : undefined,
      })
      navigate(`/sites/${site.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create site')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto animate-fade-in">
      {/* Header */}
      <div className="mb-6">
        <Link
          to="/sites"
          className="inline-flex items-center gap-2 text-muted-foreground hover:text-foreground mb-4"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Sites
        </Link>
        <h1 className="text-2xl font-bold">Create New Site</h1>
        <p className="text-muted-foreground">
          Deploy a new PHP website or application
        </p>
      </div>

      {/* Form */}
      <form onSubmit={handleSubmit} className="space-y-6">
        {error && (
          <div className="bg-red-500/10 border border-red-500/20 text-red-400 px-4 py-3 rounded-lg text-sm">
            {error}
          </div>
        )}

        <div className="bg-card border border-border rounded-xl p-6 space-y-5">
          <div className="flex items-center gap-3 pb-4 border-b border-border">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-emerald-500/20 to-emerald-600/20 flex items-center justify-center border border-emerald-500/20">
              <Globe className="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <h2 className="font-semibold">Site Details</h2>
              <p className="text-sm text-muted-foreground">Basic site information</p>
            </div>
          </div>

          <div className="space-y-2">
            <label htmlFor="name" className="block text-sm font-medium">
              Site Name
            </label>
            <input
              id="name"
              type="text"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="w-full px-4 py-2.5 bg-secondary border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 transition-colors"
              placeholder="My Awesome Site"
            />
            <p className="text-xs text-muted-foreground">
              Optional. Defaults to domain name.
            </p>
          </div>

          <div className="space-y-2">
            <label htmlFor="domain" className="block text-sm font-medium">
              Domain <span className="text-red-400">*</span>
            </label>
            <input
              id="domain"
              type="text"
              value={form.domain}
              onChange={(e) => setForm({ ...form, domain: e.target.value })}
              className="w-full px-4 py-2.5 bg-secondary border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 transition-colors"
              placeholder="example.com"
              required
            />
          </div>

          <div className="space-y-2">
            <label htmlFor="php_version" className="block text-sm font-medium">
              PHP Version
            </label>
            <select
              id="php_version"
              value={form.php_version}
              onChange={(e) => setForm({ ...form, php_version: e.target.value })}
              className="w-full px-4 py-2.5 bg-secondary border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 transition-colors"
            >
              {phpVersions.map((php) => (
                <option key={php.version} value={php.version}>
                  PHP {php.version} ({php.status})
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <label htmlFor="public_path" className="block text-sm font-medium">
              Public Directory
            </label>
            <input
              id="public_path"
              type="text"
              value={form.public_path}
              onChange={(e) => setForm({ ...form, public_path: e.target.value })}
              className="w-full px-4 py-2.5 bg-secondary border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 transition-colors"
              placeholder="public"
            />
            <p className="text-xs text-muted-foreground">
              The publicly accessible directory (e.g., "public", "web", "html")
            </p>
          </div>
        </div>

        {/* Worker Mode */}
        <div className="bg-card border border-border rounded-xl p-6 space-y-5">
          <div className="flex items-center gap-3 pb-4 border-b border-border">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-purple-500/20 to-purple-600/20 flex items-center justify-center border border-purple-500/20">
              <Zap className="w-5 h-5 text-purple-400" />
            </div>
            <div className="flex-1">
              <h2 className="font-semibold">Worker Mode</h2>
              <p className="text-sm text-muted-foreground">
                Enable for high-performance applications
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={form.worker_mode}
                onChange={(e) =>
                  setForm({ ...form, worker_mode: e.target.checked })
                }
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-secondary rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-emerald-500"></div>
            </label>
          </div>

          {form.worker_mode && (
            <div className="space-y-5 pt-2">
              <div className="space-y-2">
                <label htmlFor="worker_file" className="block text-sm font-medium">
                  Worker Script
                </label>
                <input
                  id="worker_file"
                  type="text"
                  value={form.worker_file}
                  onChange={(e) =>
                    setForm({ ...form, worker_file: e.target.value })
                  }
                  className="w-full px-4 py-2.5 bg-secondary border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 transition-colors"
                  placeholder="index.php"
                />
              </div>

              <div className="space-y-2">
                <label htmlFor="worker_num" className="block text-sm font-medium">
                  Number of Workers
                </label>
                <input
                  id="worker_num"
                  type="number"
                  min="1"
                  max="100"
                  value={form.worker_num}
                  onChange={(e) =>
                    setForm({ ...form, worker_num: parseInt(e.target.value) || 2 })
                  }
                  className="w-full px-4 py-2.5 bg-secondary border border-border rounded-lg focus:outline-none focus:ring-2 focus:ring-emerald-500/50 focus:border-emerald-500 transition-colors"
                />
                <p className="text-xs text-muted-foreground">
                  Recommended: 2-4 workers per CPU core
                </p>
              </div>

              <div className="bg-purple-500/10 border border-purple-500/20 rounded-lg p-4">
                <p className="text-sm text-purple-200">
                  <strong>Worker Mode</strong> keeps your application in memory,
                  dramatically improving performance. Ideal for Laravel, Symfony,
                  and WordPress sites.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Submit */}
        <div className="flex items-center gap-4">
          <button
            type="submit"
            disabled={isLoading || !form.domain}
            className="flex-1 py-3 px-4 bg-gradient-to-r from-emerald-500 to-emerald-600 hover:from-emerald-600 hover:to-emerald-700 text-white font-medium rounded-lg transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isLoading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Creating Site...
              </>
            ) : (
              'Create Site'
            )}
          </button>
          <Link
            to="/sites"
            className="px-6 py-3 text-muted-foreground hover:text-foreground transition-colors"
          >
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}

