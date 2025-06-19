import React, {useEffect, useState} from 'react';
import {Button, Input, Modal, Upload, Spin, Row, Col, Switch, notification, Progress, Select} from 'antd';
import {UploadOutlined} from "@ant-design/icons";
import axios from "axios";
import {getBaseUrl} from "../treeFolder";

const extractFolderName =  file => {
    let path = file.originFileObj.webkitRelativePath;
    if(path !== "" && path.indexOf("/") !== -1){
        return path.slice(0,path.indexOf("/"));
    }
    return "";
};

function getBase64(img, callback) {
    const reader = new FileReader();
    reader.addEventListener('load', () => callback(reader.result));
    reader.readAsDataURL(img);
}

const stopRequest = ({ file, onSuccess }) => {
    setTimeout(() => onSuccess("ok"), 0);
};

// singleFolderDisplay if true, only images selection and upload in a predefined folder
export default function UploadFolder({setUpdate,isAddPanelVisible,setIsAddPanelVisible,singleFolderDisplay=false,defaultPath='',callbackAfterUpload=()=>{}}) {

    const [path,setPath] = useState('');
    const [source,setSource] = useState('');
    const [waitUpload,setWaitUpload] = useState(false);
    const [progress,setProgress] = useState(0);
    const [title, setTitle] = useState('');
    const [description, setDescription] = useState('');
    const [selectDirectory,setSelectDirectory] = useState(false);
    let limitImages = 10;

    const [images,setImages] = useState([]);
    const [sources,setSources] = useState([]);

    useEffect(()=>{
        axios({
            method:'GET',
            url:`${getBaseUrl()}/sources`
        }).then(d => {
            setSources(d.data)
            setSource(d.data[0])
        })
    },[])

    const updateImages = ({fileList,file}) => {
        if(file != null && file.status === "done"){
            setPath(extractFolderName(file));
            getBase64(file.originFileObj,(base64Img)=>{
                setImages(list=>[...list,{preview:base64Img,image:file.originFileObj}])
            });
        }
    };

    const uploadPhotos = ()=>{
        setWaitUpload(true);
        let data = new FormData();
        data.append("path",singleFolderDisplay ? defaultPath:path);
        data.append("source",source);
        data.append("title",title);
        data.append("description",description);
        // Mode to upload only few images in existing folder
        if(singleFolderDisplay){
            data.append("addToFolder","true");
        }
        images.forEach((img,i)=>data.append(`file_${i}`,img.image));
        // Open notification

        axios({
            method:'POST',
            url:`${getBaseUrl()}/photo`,
            data:data,
            // Progress count for 25%
            onUploadProgress:info=>setProgress(Math.round((info.loaded / info.total)*25))
        }).then(d=>{
            // Request sended, get upload progress id and check updates
            if(d.data.status === "running") {
                monitorUpdateProgress(d.data.id, path);
            }else{
                notification["error"]({message:"Echec de la sauvegarde",description:`Erreur du serveur`});
            }
        }).catch(error=>{
            notification["error"]({message:"Echec de la sauvegarde",description:`Erreur du serveur ${error}`});
            setWaitUpload(false);
        });
    };

    const monitorUpdateProgress = (id,path)=> {
        let es = new EventSource(`/statUploadRT?id=${encodeURIComponent(id)}`);
        es.addEventListener("stat", mess => {
            let stat = JSON.parse(mess.data);
            let percent = 25 + Math.round((stat.done/stat.total)*75);
            setProgress(percent);
        });
        es.addEventListener("end", mess => {
            if(JSON.parse(mess.data).End === true){
                uploadDone(path);
            }
            es.close();
        });
        es.addEventListener("error-message", mess => {
            let message = JSON.parse(mess.data).Error;
            notification["error"]({message:"Echec de la sauvegarde",description:`${message}`});
        });
    };

    const uploadDone = path=> {
        setUpdate(u=>!u);
        setIsAddPanelVisible(false);
        setWaitUpload(false);
        setImages([]);
        setPath("");
        callbackAfterUpload();
        notification["success"]({message:'Transfert effectué',description:`Les photos ont été sauvegardées sur le serveur dans ${path}`,duration:0});
    };

    const cancelUpload = ()=> {
        setIsAddPanelVisible(false);
        setImages([]);
    };

    const changePath = field=>setPath(field.target.value);

    return (
        <Modal
            title="Ajouter des photos"
            className={"upload-photos-modal"}
            visible={isAddPanelVisible}
            onOk={uploadPhotos}
            maskClosable={false}
            bodyStyle={{height:450 + 'px'}}
            onCancel={cancelUpload}
            okText={"Envoyer"}
            cancelText={"Annuler"}
        >
            <div>
                <Progress
                    strokeColor={{
                        from: '#e99200',
                        to: '#0094d0',
                    }}
                    percent={progress}
                    status="active"
                    style={{display:waitUpload?'block':'none'}}
                />
                <Spin spinning={waitUpload}>
                    {!singleFolderDisplay ?
                        <>
                            <Row>
                                <Col style={{paddingTop:5,paddingRight:5,width:100}}>Source : </Col>
                                <Col><Select onChange={setSource} options={sources.map(d=>{return {value:d,label:d}})} style={{minWidth:350}}/></Col>
                            </Row>
                            <Row>
                                <Col style={{paddingTop:5,paddingRight:5,width:100}}>Chemin : </Col>
                                <Col><Input onChange={changePath} style={{minWidth:350}} value={path} placeholder={"Ex : 2019/current"}/></Col>
                            </Row>
                            <Row>
                                <Col style={{paddingTop:5,paddingRight:5,width:100}}>Titre : </Col>
                                <Col><Input onChange={v=>setTitle(v.target.value)} style={{minWidth:350}} value={title}/></Col>
                            </Row>
                            <Row>
                                <Col style={{paddingTop:5,paddingRight:5,width:100}}>Description : </Col>
                                <Col><Input onChange={v=>setDescription(v.target.value)} style={{minWidth:350}} value={description}/></Col>
                            </Row>
                            <Row style={{padding:5}}>
                                <Col style={{paddingTop:5,paddingRight:5}}>Mode photos</Col>
                                <Col style={{paddingTop:5,paddingRight:5}}><Switch onChange={value=>setSelectDirectory(value)}/></Col>
                                <Col style={{paddingTop:5,paddingRight:5}}>Mode répertoire</Col>
                            </Row>
                        </>:<></>
                    }
                    <Upload multiple
                            customRequest={stopRequest}
                            onChange={updateImages}
                            showUploadList={false}
                            accept={"image/*"}
                            directory={selectDirectory}
                    >
                        <Button>
                            <UploadOutlined /> {selectDirectory ? 'Choisir un répertoire':'Choisir des photos'}
                        </Button>
                        <span style={{marginLeft:10+'px'}}>
                    {images.length > limitImages ? <span>{images.length - limitImages} / </span>:''}
                            {images.length} photo(s)
                </span>
                    </Upload>
                    <div className={"upload-list"}>
                        {
                            // Show only 10 to limit browser overload
                            images.filter((img,i)=>i < limitImages).map((img,i)=><img alt="" key={`img_${i}`} src={img.preview}/>)
                        }
                        {images.length > limitImages ? '...':''}
                    </div>
                </Spin>
            </div>

        </Modal>
    )
}