import {useState} from 'react';
import logo from './assets/images/logo-universal.png';
import './App.css';
import {Status} from "../wailsjs/go/main/App";

function App() {
    const [resultText, setResultText] = useState("Please enter your name below 👇");
    const [name, setName] = useState('');
    const updateName = (e: any) => setName(e.target.value);
    const updateResultText = (result: string) => setResultText(result);

    const status = async () => {
        const daemonStatus = await Status();
        if (daemonStatus.success) {
            setResultText(daemonStatus.data);
        } else {
            setResultText(daemonStatus.error);
        }
    }


    return (
        <div id="App">
            <img src={logo} id="logo" alt="logo"/>
            <div id="result" className="result">{resultText}</div>
            <div id="input" className="input-box">
                <input id="name" className="input" onChange={updateName} autoComplete="off" name="input" type="text"/>
                <button className="btn" onClick={status}>Greet</button>
            </div>
        </div>
    )
}

export default App
