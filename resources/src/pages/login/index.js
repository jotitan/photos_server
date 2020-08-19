import React, {useState} from 'react';
import 'moment/locale/fr';
import {Col, Input, Modal, Row} from 'antd';

export default function Login({setCanAdmin,setShowPanelLogin,showPanelLogin,checkAdminAccess}) {
    const [username,setUsername] = useState('');
    const [password,setPassword] = useState('');
    const [message,setMessage] = useState('');

    const checkAccess = ()=>{
        checkAdminAccess(setCanAdmin,setShowPanelLogin,true,{username:username,password:password})
            .then(()=>setShowPanelLogin(false))
            .catch(()=>setMessage("Impossible de se connecter, mauvais login / mot de passe"))
    }
    return <>
        {showPanelLogin ?
        <Modal
            title="Login"
            className={"upload-photos-modal"}
            visible={showPanelLogin}
            onOk={()=>checkAccess()}
            onCancel={()=>setShowPanelLogin(false)}
            maskClosable={false}
            okText={"Se connecter"}
            cancelText={"Lecture seule"}
        >
            {message !== '' ? <Row style={{color:'red',fontWeight:'bold'}}>{message}</Row>:''}
            <Row style={{marginBottom:10}}>
                <Col flex={"100px"} style={{paddingLeft:10}}>Login</Col>
                <Col><Input onChange={e=>setUsername(e.target.value)} style={{width:'180px'}}/></Col>
            </Row>
            <Row>
                <Col flex={"100px"} style={{paddingLeft:10}}>Mot de passe</Col>
                <Col><Input.Password onChange={e=>setPassword(e.target.value)} style={{width:'180px'}}/></Col>
            </Row>
        </Modal>:<></>}</>;

}