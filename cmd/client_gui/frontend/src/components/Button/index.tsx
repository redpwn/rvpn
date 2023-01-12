import "./style.css";

interface ButtonProps {
  onClick(): any;
  text: string;

  //optional styling props
  disabled?: boolean;
}

const Button = (props: ButtonProps) => {
  if (props.disabled) {
    // the button should be disabled
    return <button className="styled-button-disabled">{props.text}</button>;
  } else {
    return (
      <button onClick={props.onClick} className="styled-button">
        {props.text}
      </button>
    );
  }
};

export default Button;
