import React, {useEffect, useState} from 'react'
import {Image, Button, Col, Modal, notification, Popconfirm, Row, Tooltip} from 'antd'
import axios from "axios";
import {getBaseUrl} from "../treeFolder";
import default_icon from './flem.png';

import {CloseOutlined, DeleteFilled, DeleteTwoTone, ChromeOutlined,PlayCircleOutlined} from "@ant-design/icons";
import ReactPlayer from "react-player/";

// setIsAddFolderPanelVisible to show folder to upload
export default function VideoDisplay({urlVideo,setUpdate}) {
    let baseUrl = getBaseUrl();
    const [videos,setVideos] = useState([]);
    const [currentVideo,setCurrentVideo] = useState(null);
    const [showVideo,setShowVideo] = useState(false);
    const [removeFolderUrl,setRemoveFolderUrl] = useState('');
    const [updateExifFolderUrl,setUpdateExifFolderUrl] = useState('');
    const loadVideos = url=>{
        axios({
            url:url,
            method:'GET'
        }).then(data=>{
            setRemoveFolderUrl(data.data.RemoveFolderUrl);
            setUpdateExifFolderUrl(data.data.UpdateExifFolderUrl);
            setVideos(data.data.Children || data.data.Files);
        });
    };

    useEffect(()=>{
        if(urlVideo !== '') {
            loadVideos(urlVideo)
        }
    },[urlVideo])

    const updateExif = ()=>{
        axios({
            url:`${baseUrl}/${updateExifFolderUrl}`
        }).then(()=>{
            setUpdate(true);
            setRemoveFolderUrl('');
            notification["success"]({message:'Répertoire mise à jour',description:'Les exifs sont à jour'});
        }).catch((e)=>
            notification["error"]({message:'Erreur de mise à jour',description:'Les exifs n\'ont pas été mises à jour ' + e})
        )
    }

    const removeFolder = ()=>{
        axios({
            url:`${baseUrl}/${removeFolderUrl}`
        }).then(()=>{
            setUpdate(true);
            setRemoveFolderUrl('');
            notification["success"]({message:'Répertoire supprimé',description:'Le répertoire a été supprimé'});
        }).catch((e)=>
            notification["error"]({message:'Erreur de suppression',description:'Le répertoire n\'a pas été supprimé ' + e})
        )
    }

    const deleteVideo = (deletePath,path)=> {
        axios({
            url:`${baseUrl}/${deletePath}`
        }).then(()=>{
            setVideos(videos.filter(v=>v.VideosPath !== path));
            notification["success"]({message:'Suppression réussie',description:'La vidéo a bien été supprimée'})
        }).catch(()=>notification["error"]({message:'Opération impossible',description:'La vidéo n\'a pas été supprimée'}))
    };
    return (
        <>
            <Row className={"options"}>
                <Col>
                    {videos != null ? videos.length:'0'} vidéo(s)
                    {removeFolderUrl !== '' && urlVideo !=='' && (videos == null || videos.length === 0) ?
                        <span style={{marginLeft:20}}>
                        <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer ce répertoire vide"}
                                    onConfirm={removeFolder} okText="Oui" cancelText="Non">
                        <Tooltip key={"image-info"} placement="top" title={"Supprimer le répertoire"}>
                            <DeleteTwoTone style={{cursor:'pointer',padding:'4px',backgroundColor:'#ff8181'}} twoToneColor={"#b32727"}/>
                        </Tooltip>
                    </Popconfirm>
                        </span>:''}
                    {updateExifFolderUrl !== '' && urlVideo !==''  ?
                        <span style={{marginLeft:20}}>
                        <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre a jour les exifs"}
                                    onConfirm={updateExif} okText="Oui" cancelText="Non">
                        <Tooltip key={"image-info"} placement="top" title={"Mise à jour des exifs"}>
                            <ChromeOutlined style={{cursor:'pointer',padding:'4px',backgroundColor:'#ff8181'}} twoToneColor={"#b32727"}/>
                        </Tooltip>
                    </Popconfirm>
                        </span>:''}
                </Col>

            </Row>
            <Row className={"video-gallery"}>
                <Col span={24} style={{marginTop:36+'px',marginLeft:15+'px'}}>
                    {
                        videos.map(video=>
                            <Row className={"video"}>
                                <Col>
                                    <Image src={video.CoverPath} fallback={default_icon} style={{width:300}}/>
                                    <PlayCircleOutlined className={"play"} onClick={()=>{
                                        setCurrentVideo(video);
                                        setShowVideo(true);
                                    }}/>
                                </Col>
                                <Col flex={"auto"} style={{color:"white",padding:20}}>
                                    <Row>
                                        <Col className={"title"}>Titre</Col>
                                        <Col>{video.Metadata.Title}</Col>
                                    </Row>
                                    <Row>
                                        <Col className={"title"}>Date</Col>
                                        <Col>{new Date(video.Metadata.Date).toLocaleString()}</Col>
                                    </Row>
                                    <Row>
                                        <Col className={"title"}>Durée</Col>
                                        <Col>{video.Metadata.Duration} s</Col>
                                    </Row>
                                    <Row>
                                        <Col className={"title"}>Mots-clés</Col>
                                        <Col>{video.Metadata.Keywords != null ? video.Metadata.Keywords.join(', '):''}</Col>
                                    </Row>
                                    <Row>
                                        <Col className={"title"}>Lieu</Col>
                                        <Col>{video.Metadata.Place != null ? video.Metadata.Place.join(", "):''}</Col>
                                    </Row>
                                    <Row>
                                        <Col className={"title"}>Qui</Col>
                                        <Col>{video.Metadata.Peoples.join(", ")}</Col>
                                    </Row>
                                    <Row>
                                        <Col span={10}>
                                            <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer cette vidéo"}
                                                        onConfirm={()=>deleteVideo(video.DeletePath,video.VideosPath)} okText="Oui" cancelText="Non">
                                                <Button>
                                                    <DeleteFilled/>Supprimer
                                                </Button>
                                            </Popconfirm>
                                        </Col>
                                    </Row>
                                </Col>
                            </Row>
                        )
                    }
                </Col>
            </Row>
            <Modal
                className={"modal-player"}
                maskStyle={{backgroundColor:'black',opacity:0.8}}
                bodyStyle={{backgroundColor:'rgba(0,0,0,0.6)',height:'100%'}}
                visible={showVideo}
                closable={true}
                onCancel={(()=>{
                    setCurrentVideo(null);
                    setShowVideo(false);
                })}
                closeIcon={<CloseOutlined style={{color:'white',position:'relative',top:-10,right:-15}}/>}

                style={{top:20,height:'90vh',margin:'20px auto'}}
                width={"65vw"}
            >
                {
                    currentVideo != null ?
                        <div style={{width:'100%',height:'100%',margin:10}}>
                            <ReactPlayer
                                playsinline
                                controls={true}
                                width={"100%"}
                                height={"100%"}
                                url={`${baseUrl}${currentVideo.VideosPath}`}
                                config={
                                    { file:{forceHLS:true}}
                                }/>
                        </div>
                        :<>Pas de vidéo</>
                }
            </Modal>
        </>
    )
}