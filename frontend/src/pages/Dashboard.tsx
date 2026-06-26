import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'

interface Endpoint {
  id: string
  name: string
  url: string
  http_method: string
  last_is_healthy: boolean
  last_response_time_ms: number
}

interface DashboardProps {
  setIsAuthenticated: (value: boolean) => void
}

export default function Dashboard({ setIsAuthenticated }: DashboardProps) {
  const [endpoints, setEndpoints] = useState<Endpoint[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showAddForm, setShowAddForm] = useState(false)
  const [formData, setFormData] = useState({
    url: '',
    http_method: 'GET',
    expected_status_code: '200',
    name: '',
  })
  const navigate = useNavigate()

  useEffect(() => {
    fetchEndpoints()
  }, [])

  const fetchEndpoints = async () => {
    try {
      const token = localStorage.getItem('jwt_token')
      const response = await axios.get('/api/v1/endpoints', {
        headers: { Authorization: `Bearer ${token}` },
      })
      setEndpoints(response.data || [])
    } catch (err: any) {
      if (err.response?.status === 401) {
        localStorage.removeItem('jwt_token')
        setIsAuthenticated(false)
        navigate('/login')
      } else {
        setError(err.response?.data?.error || 'Failed to fetch endpoints')
      }
    } finally {
      setLoading(false)
    }
  }

  const handleAddEndpoint = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    try {
      const token = localStorage.getItem('jwt_token')
      const payload = {
        name: formData.name,
        url: formData.url,
        http_method: formData.http_method,
        expected_status_code: parseInt(formData.expected_status_code, 10),
      }
      await axios.post('/api/v1/endpoints', payload, {
        headers: { Authorization: `Bearer ${token}` },
      })
      setFormData({ url: '', http_method: 'GET', expected_status_code: '200', name: '' })
      setShowAddForm(false)
      await fetchEndpoints()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to add endpoint')
    }
  }

  const handleLogout = () => {
    localStorage.removeItem('jwt_token')
    setIsAuthenticated(false)
    navigate('/login')
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-6xl mx-auto px-4 py-4 flex justify-between items-center">
          <h1 className="text-2xl font-bold text-gray-900">API Uptime Monitor</h1>
          <button onClick={handleLogout} className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition">
            Logout
          </button>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-4 py-8">
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="bg-white p-6 rounded-lg shadow">
            <p className="text-gray-600 text-sm">Total Endpoints</p>
            <p className="text-3xl font-bold text-gray-900">{endpoints.length}</p>
          </div>
          <div className="bg-white p-6 rounded-lg shadow">
            <p className="text-gray-600 text-sm">Healthy</p>
            <p className="text-3xl font-bold text-green-600">{endpoints.filter((e) => e.last_is_healthy).length}</p>
          </div>
          <div className="bg-white p-6 rounded-lg shadow">
            <p className="text-gray-600 text-sm">Down</p>
            <p className="text-3xl font-bold text-red-600">{endpoints.filter((e) => !e.last_is_healthy).length}</p>
          </div>
        </div>

        <div className="mb-6">
          <button
            onClick={() => setShowAddForm(!showAddForm)}
            className="bg-primary-600 text-white px-4 py-2 rounded-lg font-medium hover:bg-primary-700 transition"
          >
            {showAddForm ? 'Cancel' : '+ Add Endpoint'}
          </button>
        </div>

        {showAddForm && (
          <div className="bg-white p-6 rounded-lg shadow mb-8">
            <h2 className="text-xl font-bold text-gray-900 mb-4">Add New Endpoint</h2>
            <form onSubmit={handleAddEndpoint} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Name (Optional)</label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600"
                    placeholder="e.g., API Health Check"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">HTTP Method</label>
                  <select
                    value={formData.http_method}
                    onChange={(e) => setFormData({ ...formData, http_method: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600"
                  >
                    <option>GET</option>
                    <option>POST</option>
                    <option>HEAD</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">URL</label>
                <input
                  type="url"
                  value={formData.url}
                  onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600"
                  placeholder="https://api.example.com/health"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Expected Status Code</label>
                <input
                  type="number"
                  value={formData.expected_status_code}
                  onChange={(e) => setFormData({ ...formData, expected_status_code: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600"
                  placeholder="200"
                  required
                />
              </div>

              {error && <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">{error}</div>}

              <button type="submit" className="w-full bg-primary-600 text-white py-2 rounded-lg font-medium hover:bg-primary-700 transition">
                Add Endpoint
              </button>
            </form>
          </div>
        )}

        {error && !showAddForm && <div className="p-4 bg-red-50 border border-red-200 text-red-700 rounded-lg mb-4">{error}</div>}

        {loading ? (
          <div className="text-center text-gray-600">Loading endpoints...</div>
        ) : endpoints.length === 0 ? (
          <div className="bg-white p-8 rounded-lg shadow text-center">
            <p className="text-gray-600 mb-4">No endpoints yet. Add one to get started!</p>
          </div>
        ) : (
          <div className="space-y-4">
            {endpoints.map((endpoint) => (
              <div key={endpoint.id} className="bg-white p-6 rounded-lg shadow">
                <div className="flex justify-between items-start mb-3">
                  <div>
                    <h3 className="text-lg font-bold text-gray-900">{endpoint.name || endpoint.url}</h3>
                    <p className="text-gray-600 text-sm">{endpoint.url}</p>
                  </div>
                  <div
                    className={`px-3 py-1 rounded-full text-sm font-medium ${
                      endpoint.last_is_healthy ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                    }`}
                  >
                    {endpoint.last_is_healthy ? '✅ Healthy' : '🚨 Down'}
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4 text-sm">
                  <div>
                    <p className="text-gray-600">Method</p>
                    <p className="font-medium text-gray-900">{endpoint.http_method}</p>
                  </div>
                  <div>
                    <p className="text-gray-600">Response Time</p>
                    <p className="font-medium text-gray-900">{endpoint.last_response_time_ms}ms</p>
                  </div>
                  <div>
                    <p className="text-gray-600">Status</p>
                    <p className="font-medium text-gray-900">OK</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
