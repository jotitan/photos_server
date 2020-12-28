import React, {useState} from 'react';
import {Button, Col, Input, Modal, notification, Progress, Row, Spin, Upload} from 'antd';
import {UploadOutlined, VideoCameraOutlined} from "@ant-design/icons";
import axios from "axios";
import {getBaseUrl} from "../treeFolder";

function getBase64(img, callback) {
    const reader = new FileReader();
    reader.addEventListener('load', () => callback(reader.result));
    reader.readAsDataURL(img);
}

const stopRequest = ({ file, onSuccess }) => {
    setTimeout(() => onSuccess("ok"), 0);
};

// singleFolderDisplay if true, only images selection and upload in a predefined folder
export default function UploadVideos({setUpdate,isAddPanelVisible,setIsAddPanelVisible,singleFolderDisplay=false,defaultPath='',callbackAfterUpload=()=>{}}) {

    const [path,setPath] = useState('');
    const [waitUpload,setWaitUpload] = useState(false);
    const [progress,setProgress] = useState(0);

    const [videos,setVideos] = useState([]);
    const [cover,setCover] = useState({});

    const updateVideo = ({fileList,file}) => {
        if(file != null && file.status === "done"){
            // If image, set the cover
            if(file.type.indexOf('image') === 0){
                // cover case
                getBase64(file.originFileObj,(base64Img)=>{
                    setCover({preview:base64Img,image:file.originFileObj})
                });
            }else{
                // video case
                setVideos(list=>[...list,file.originFileObj])
            }
        }
    };

    const uploadVideo = ()=>{
        setWaitUpload(true);
        let data = new FormData();
        data.append("path",path);
        videos.forEach((video,i)=>data.append(`file_${i}`,video));
        // Put image at the end
        data.append(`has_cover`, cover.image != null);
        if(cover.image != null) {
            data.append(`file_${videos.length}`, cover.image);
        }
        // Open notification

        axios({
            method:'POST',
            url:getBaseUrl()+'/uploadVideo',
            data:data,
            // Progress count for 50%
            onUploadProgress:info=>setProgress(Math.round((info.loaded / info.total)*50))
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
            let percent = 50 + Math.round((stat.done/stat.total)*50);
            setProgress(percent);
        });
        es.addEventListener("end", mess => {
            console.log("end",mess)
            if(JSON.parse(mess.data).End === true){
                uploadDone(path);
            }
            es.close();
        });
        es.addEventListener("error-message", mess => {
            let message = JSON.parse(mess.data).Error;
            notification["error"]({message:"Echec de la sauvegarde",description:message});
            es.close();
            uploadDone("",false);
        });
    };

    const uploadDone = (path,success=true)=> {
        setUpdate(u=>!u);
        setIsAddPanelVisible(false);
        setWaitUpload(false);
        setVideos([]);
        setCover({});
        setPath("");
        if(success) {
            callbackAfterUpload();
            notification["success"]({
                message: 'Transfert effectué',
                description: `La vidéo a été sauvegardée sur le serveur dans ${path}`,
                duration: 0
            });
        }
    };

    const cancelUpload = ()=> {
        setIsAddPanelVisible(false);
        setVideos([]);
        setCover({});
    };

    const changePath = field=>setPath(field.target.value);

    return (
        <Modal
            title="Ajouter des vidéos"
            className={"upload-photos-modal"}
            visible={isAddPanelVisible}
            onOk={uploadVideo}
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
                    <Row>
                        <Col style={{paddingTop:5+'px',paddingRight:5+'px'}}>Chemin : </Col>
                        <Col><Input onChange={changePath} style={{minWidth:350+'px'}} value={path} placeholder={"Ex : 2019/current"}/></Col>
                    </Row>
                    <Upload multiple
                            customRequest={stopRequest}
                            onChange={updateVideo}
                            showUploadList={false}
                            accept={"video/*,image/*"}
                    >
                        Il faut sélectionner les différentes résolutions et une photo
                        <Button>
                            <UploadOutlined /> Choisir une vidéo
                        </Button>
                        <span style={{marginLeft:10+'px'}}>{videos.length} vidéo(s)</span>
                    </Upload>
                    <div className={"upload-list-video"}>
                        {videos.map(video=><div><VideoCameraOutlined /> {video.name}</div>)}
                        {cover.image != null ? <img alt="" key={`cover`} src={cover.preview}/>:<></>}
                    </div>
                </Spin>
            </div>

        </Modal>
    )
}