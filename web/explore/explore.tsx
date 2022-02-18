import React from 'react'
import { render } from 'react-dom'
import { GraphiQL } from 'graphiql'
import 'graphiql/graphiql.css'

// const Logo = function Logo(): JSX.Element {
//   return <span>My Corp</span>
// }

// Logo.displayName = 'logo'

// // See GraphiQL Readme - Advanced Usage section for more examples like this
// GraphiQL.Logo = Logo

const App = (): JSX.Element => { 
  const path = location.host + location.pathname.replace(/\/explore.*$/, '')


  return <div style={{ height: '100vh' }}>
    <GraphiQL
      headerEditorEnabled
      fetcher={async (graphQLParams) => {
        const resp = await fetch(
          location.protocol + '//' + path,
          {
            method: 'POST',
            headers: {
              Accept: 'application/json',
              'Content-Type': 'application/json',
            },
            body: JSON.stringify(graphQLParams),
            credentials: 'same-origin',
          },
        )
        return resp.json().catch(() => resp.text())
      }}
    />
  </div>
}

render(<App />, document.getElementById('root'))
