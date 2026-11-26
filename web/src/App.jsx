import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout.jsx'
import Login from './pages/Login.jsx'
import Dashboard from './pages/Dashboard.jsx'
import Logs from './pages/Logs.jsx'
import Sites from './pages/Sites.jsx'
import SiteDetail from './pages/SiteDetail.jsx'
import SiteStats from './pages/SiteStats.jsx'
import Templates from './pages/Templates.jsx'
import Certificates from './pages/Certificates.jsx'
import Users from './pages/Users.jsx'
import Tenants from './pages/Tenants.jsx'
import CrowdSec from './pages/CrowdSec.jsx'
import Audit from './pages/Audit.jsx'
import IPLists from './pages/IPLists.jsx'
import Settings from './pages/Settings.jsx'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="logs" element={<Logs />} />
          <Route path="sites" element={<Sites />} />
          <Route path="sites/stats" element={<SiteStats />} />
          <Route path="sites/:id" element={<SiteDetail />} />
          <Route path="templates" element={<Templates />} />
          <Route path="certificates" element={<Certificates />} />
          <Route path="users" element={<Users />} />
          <Route path="tenants" element={<Tenants />} />
          <Route path="crowdsec" element={<CrowdSec />} />
          <Route path="iplist" element={<IPLists />} />
          <Route path="audit" element={<Audit />} />
          <Route path="settings" element={<Settings />} />
        </Route>
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </BrowserRouter>
  )
}
