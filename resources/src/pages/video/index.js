import React, {useEffect, useState} from 'react'
import {Image,Button, Col, Modal, notification, Popconfirm, Row} from 'antd'
import axios from "axios";
import {getBaseUrl} from "../treeFolder";
import default_icon from './flem.png';

import {DeleteFilled, PlayCircleOutlined} from "@ant-design/icons";
import ReactPlayer from "react-player/";

// setIsAddFolderPanelVisible to show folder to upload
export default function VideoDisplay({urlVideo}) {
    let baseUrl = getBaseUrl();
    const [videos,setVideos] = useState([]);
    const [currentVideo,setCurrentVideo] = useState(null);
    const [showVideo,setShowVideo] = useState(false);
    const loadVideos = url=>{
        axios({
            url:url,
            method:'GET'
        }).then(data=>setVideos(data.data));
    };

    useEffect(()=>{
        if(urlVideo !== '') {
            loadVideos(urlVideo)
        }
    },[urlVideo])

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
                                        <Col>{video.Metadata.Place}</Col>
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
                bodyStyle={{backgroundColor:'black'}}
                visible={showVideo}
                closable={true}
                onCancel={(()=>{
                    setCurrentVideo(null);
                    setShowVideo(false);
                })}
                style={{color:'white'}}
                width={"60%"}
            >
                {
                    currentVideo != null ?
                        <div className="container">
                            <ReactPlayer
                                playsinline
                                controls={true}
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