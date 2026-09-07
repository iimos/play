import React from 'react'

export interface LayoutProps {
  title: string
  children: React.ReactNode
  style?: React.CSSProperties
}

const Layout: React.FC<LayoutProps> = ({ title, children, style }) => {
  return (
    <div className="k-line-chart-container" style={style}>
      <h3
        className="k-line-chart-title">{title}</h3>
      {children}
    </div>
  )
}


export default Layout