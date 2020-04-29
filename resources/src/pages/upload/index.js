import React, {useState} from 'react';
import {Button, Input, Modal, Upload, Spin, Row, Col, Switch} from 'antd';
import {UploadOutlined} from "@ant-design/icons";
import axios from "axios";
import {getBaseUrl} from "../treeFolder";

export default function UploadFolder({setUpdate,isAddPanelVisible,setIsAddPanelVisible}) {

    const [path,setPath] = useState('');
    const [waitUpload,setWaitUpload] = useState(false);
    const [selectDirectory,setSelectDirectory] = useState(false);
    let limitImages = 10;
    const stopRequest = ({ file, onSuccess }) => {
        setTimeout(() => onSuccess("ok"), 0);
    };

    function getBase64(img, callback) {
        const reader = new FileReader();
        reader.addEventListener('load', () => callback(reader.result));
        reader.readAsDataURL(img);
    }

    const [images,setImages] = useState([]);

    const extractFolderName =  file => {
        let path = file.originFileObj.webkitRelativePath;
        if(path !== "" && path.indexOf("/") !== -1){
             return path.slice(0,path.indexOf("/"));
        }
        return "";
    };

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
        data.append("path",path)
        images.forEach((img,i)=>data.append(`file_${i}`,img.image));
        axios({
            method:'POST',
            url:getBaseUrl()+'/uploadFolder',
            data:data
        }).then(d=>{
            // Loaded
            uploadDone()
        });
    };

    const uploadDone = ()=> {
        setUpdate(u=>!u);
        setIsAddPanelVisible(false);
        setWaitUpload(false);
        setImages([]);
        setPath("");
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
            onCancel={()=>setIsAddPanelVisible(false)}
            okText={"Envoyer"}
            cancelText={"Annuler"}
        >
            <div>
                <Spin spinning={waitUpload}>
                    <Row>
                        <Col style={{paddingTop:5+'px',paddingRight:5+'px'}}>Chemin : </Col>
                        <Col><Input onChange={changePath} value={path} placeholder={"Ex : 2019/current"}/></Col>
                    </Row>
                    <Row style={{padding:5+'px'}}>
                        <Col style={{paddingTop:5+'px',paddingRight:5+'px'}}>Mode photos</Col>
                        <Col style={{paddingTop:5+'px',paddingRight:5+'px'}}><Switch onChange={value=>setSelectDirectory(value)}/></Col>
                        <Col style={{paddingTop:5+'px',paddingRight:5+'px'}}>Mode répertoire</Col>
                    </Row>
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