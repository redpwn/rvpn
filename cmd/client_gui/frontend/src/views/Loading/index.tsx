import { BarLoader } from "react-spinners";
import logo from "../..//assets/images/logo_w.svg";

import "./style.css";

const Loading = () => {
  return (
    <div className="loading-container">
      <img src={logo} className="logo" />
      <BarLoader color="#E43A49" />
    </div>
  );
};

export default Loading;
