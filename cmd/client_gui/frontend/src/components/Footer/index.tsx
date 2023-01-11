import { useEffect, useState } from "react";
import { Version } from "../../../wailsjs/go/main/App";
import "./style.css";

const Footer = () => {
  const [rVPNVersion, setRVPNVersion] = useState("");

  useEffect(() => {
    const fetchVersion = async () => {
      const respVersion = await Version();
      setRVPNVersion(respVersion);
    };

    fetchVersion().catch(console.error);
  }, []);

  return (
    <div className="footer-container">
      <span>rVPN version {rVPNVersion}</span>
    </div>
  );
};

export default Footer;
