import React, {useState} from "react";
import './edit-title.css';
import axios from "axios";
import {getBaseUrl} from "../../pages/treeFolder";
import {Col, Input, Modal, notification, Row} from "antd";
import {useTranslation} from "react-i18next";
import TextArea from "antd/es/input/TextArea";

function sendRequest(folder, title, description){
    const data = {Path:folder,Title:title, Description:description}
    return axios({
        url:`${getBaseUrl()}/photo/folder/edit-details`,
        method:'POST',
        data:JSON.stringify(data)
    })
}

export default function EditTitle({folder, initialValues, close}) {

    const [title, setTitle] = useState(initialValues.Title);
    const [description, setDescription] = useState(initialValues.Description);
    const {t} = useTranslation();

    const updateDetails = () => {
        sendRequest(folder, title, description)
            .then(() => {
                notification["success"]({message:'Mise à jour effectuée',description:'Les détails du répertoire ont été mis à jour'})
                close()
            })
            .catch(e=>notification["error"]({message:'Erreur de mise à jour',description:'Impossible de sauvegarder les détails ' + e}))
    }
    return <div>

        <Modal title={t('details.edit')} onOk={updateDetails} onCancel={close} open={true}>
            <Row style={{marginBottom:10}}>
                <Col flex={"100px"} style={{paddingLeft:10}}>{t('details.title')}</Col>
                <Col><Input onChange={e=>setTitle(e.target.value)} style={{width:'370px'}} value={title}/></Col>
            </Row>
            <Row style={{marginBottom:10}}>
                <Col flex={"100px"} style={{paddingLeft:10}}>{t('details.description')}</Col>
                <Col><TextArea onChange={e=>setDescription(e.target.value)} style={{width:'370px'}} value={description} rows={3}/></Col>
            </Row>
        </Modal>
    </div>
}

