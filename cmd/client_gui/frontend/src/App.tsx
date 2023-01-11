import { useState, useEffect } from "react";
import { GetControlPlaneAuth } from "../wailsjs/go/main/App";
import Loading from "./views/Loading";
import Login from "./views/Login";
import Home from "./views/Home";

const App = () => {
  const [loading, setLoading] = useState(true);
  const [auth, setAuth] = useState("");

  useEffect(() => {
    const fetchAuth = async () => {
      const controlPlaneAuth = await GetControlPlaneAuth();
      if (controlPlaneAuth.success) {
        setAuth(controlPlaneAuth.data);
        setLoading(false);
      } else {
        // something went wrong
        throw new Error(controlPlaneAuth.error);
      }
    };

    fetchAuth().catch(console.error);
  }, []);

  if (loading) {
    return (
      <div className="app">
        <Loading />
      </div>
    );
  } else {
    if (auth === "") {
      // user is not logged in, we display login page
      return <Login setAuth={setAuth} />;
    } else {
      // user is logged in, we display home page
      return <Home setAuth={setAuth} />;
    }
  }
};

export default App;
