import "./style.css";

interface ButtonProps {
  onClick(): any;
  text: string;

  //optional styling props
}

const Button = (props: ButtonProps) => {
  return (
    <button onClick={props.onClick} className="styled-button">
      {props.text}
    </button>
  );
};

export default Button;
